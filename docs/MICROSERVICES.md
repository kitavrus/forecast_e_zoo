# e_zoo: 7 микросервисов

Платформа e_zoo разделена на 7 независимых binary-сервисов. Все они
делят одну Postgres-инстанцию (БД `source_adapter`, разные schema)
и общий JWT-секрет для service-to-service вызовов.

## Карта сервисов

| # | Service          | Port | Cron       | Module | Назначение                                |
|---|------------------|------|------------|--------|-------------------------------------------|
| 1 | source-adapter   | 8080 | `0 2 * * *`  | M1     | Загрузка фактов/мастер-данных из ERP      |
| 2 | etl              | 8081 | `30 2 * * *` | M2     | Извлечение → валидация → марты            |
| 3 | data-marts       | 8082 | —          | M3     | Read-only API над `marts.*`               |
| 4 | kpi              | 8083 | `0 4 * * *`  | M4     | Расчёт KPI, калибровка                    |
| 5 | forecast         | 8084 | `0 5 * * *`  | M5     | Прогноз спроса + replenishment plans      |
| 6 | order-builder    | 8086 | `0 6 * * *`  | M6     | Сборка purchase orders из планов          |
| 7 | channel-router   | 8087 | `30 6 * * *` | M7     | Отправка PO в каналы поставщиков          |

> Порт **8085 пропущен** — занят другим инструментом локально.

## Cron timeline (Europe/Kyiv)

```
02:00  source-adapter   ── загрузка ERP-данных (snapshot N)
02:30  etl              ── extract → validate → load в marts.*
04:00  kpi              ── KPI snapshots на основе marts.*
05:00  forecast         ── forecast runs + replenishment plans
06:00  order-builder    ── plans → purchase_orders
06:30  channel-router   ── purchase_orders → отправка в каналы
```

Каждый сервис имеет advisory-lock в Postgres, поэтому запуск через
`/admin/*` API во внеплановое время не конфликтует с cron.

## Cross-service communication

| Откуда            | Куда             | Транспорт | Auth                 |
|-------------------|------------------|-----------|----------------------|
| etl               | source-adapter   | HTTP      | JWT (role x-flow-etl) |
| kpi               | postgres (marts) | DB        | роль `mart_reader`   |
| forecast          | postgres (marts) | DB        | роль `mart_reader`   |
| order-builder     | postgres         | DB        | `e_zoo_app`          |
| channel-router    | postgres + ERP   | DB + HTTP | `e_zoo_app` + per-channel auth |
| Внешние клиенты   | data-marts       | HTTP      | JWT                  |

## Database

Один Postgres-кластер 18, БД `source_adapter`. Schema:

| Schema     | Owner   | Содержимое                        |
|------------|---------|-----------------------------------|
| public     | adapter | M1 (master + facts partitioned)   |
| marts      | adapter | M2 (read marts), `mart_reader` ro |
| kpi        | adapter | M4 snapshots / calibrations       |
| forecast   | adapter | M5 forecast_runs / plans          |
| orders     | adapter | M6 purchase orders                |
| channels   | adapter | M7 send_attempts / channel_config |

Init script `infra/pg/init/01_init.sql` создаёт:
- роль `e_zoo_app` (login) — общая для всех 7 сервисов (PoC)
- роль `mart_reader` (NOLOGIN) — для GRANT'ов на marts.*

## Запуск через docker-compose

```bash
cp .env.example .env
docker compose up -d --build
```

Порядок запуска (через `depends_on`):
1. **postgres** (healthcheck pg_isready)
2. **migrate** (одноразово, прогоняет все 9 миграций)
3. **source-adapter** (зависит от migrate completed)
4. **etl** (зависит от source-adapter started + migrate completed)
5. **data-marts, kpi, forecast, order-builder, channel-router** (параллельно после migrate)

### Smoke check всех сервисов

```bash
for port in 8080 8081 8082 8083 8084 8086 8087; do
  echo -n "port $port: "; curl -fsS -m 3 http://localhost:$port/healthz || echo
done
```

### Логи конкретного сервиса

```bash
make compose-logs SERVICE=channel-router
```

### Остановка

```bash
make compose-down       # docker compose down -v
```

## Локальный запуск (без Docker)

```bash
# 1. Запустить postgres локально (или docker compose up -d postgres)
# 2. Применить все миграции
make migrate-up-all
# 3. Запустить нужный сервис
make run-source-adapter
make run-etl
make run-kpi
# ...
```

## Сборка binary

```bash
make build-all                # собрать все 7 binary в bin/
make build-channel-router     # только channel-router
```

## Migrations flow

| Префикс | Feature        | Schema    | Назначение                |
|---------|----------------|-----------|---------------------------|
| 0001    | data_export    | public    | master + service tables   |
| 0002    | data_export    | public    | partitioned facts         |
| 1001    | etl_validation | marts     | marts schema              |
| 1002    | etl_validation | marts     | etl_runs table            |
| 2001    | kpi            | kpi       | kpi schema + tables       |
| 3001    | forecast       | forecast  | forecast_runs / plans     |
| 4001    | orders         | orders    | purchase_orders           |
| 5001    | channels       | channels  | send_attempts / config    |

Все миграции собираются `Dockerfile.migrate` в плоскую директорию
`/migrations` и применяются `golang-migrate` строго по числовому
префиксу.

## Quality gates

```bash
go build ./...              # сборка всех 7 binary
go vet ./...                # static analysis
make test                   # unit tests
make test-integration       # integration (requires Docker)
make lint                   # golangci-lint
```

## E2E тест с mock ERP

Полный pipeline (mock-ERP → 7 микросервисов → mock-ERP) проверяется одним bash-скриптом:

```bash
make e2e-up                                  # up + seed_channel_configs
bash tests/e2e/run.sh --skip-up              # 8 stages × admin endpoints
```

Скрипт прогоняет 8 стадий (source-adapter ingest → etl marts → kpi → forecast → approve plans → order-builder → channel-router → mock-erp verify), на каждой делает HTTP-вызов админ-эндпоинта и проверяет соответствующую таблицу в Postgres.

Подробности (per-stage breakdown, флаги, troubleshooting) — [docs/E2E-TEST.md](E2E-TEST.md).

## Известные проблемы

- **Миграция `4001_orders_schema.up.sql` (M6, orders)** — pre-existing bug:
  `CREATE UNIQUE INDEX uq_purchase_orders_po_number ON orders.purchase_orders(po_number)` падает
  на partitioned table, потому что unique index должен включать partition key
  `created_at`. Это блокирует `docker compose up` (migrate-сервис падает с exit 1),
  и зависимые от migrate сервисы (M1, M2, M3, M4, M5, M6, M7) не стартуют.
  Postgres + cm-stage `Dockerfile.migrate` сборка работают; миграции 0001..3001
  проходят. **Чинится отдельной задачей** — нужно либо изменить unique index на
  `(po_number, created_at)`, либо вынести `po_number` в отдельную не-partitioned
  справочную таблицу.
