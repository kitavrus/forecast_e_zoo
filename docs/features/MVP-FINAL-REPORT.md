# MVP Pipeline Final Report
**Дата:** 2026-05-07
**Module path:** github.com/Kitavrus/e_zoo
**Стек:** Go 1.26 + Fiber v3 + pgx/v5 + go:embed + golang-migrate + dockertest + gocron v2 + Prometheus

## Что сделано

Реализован полный MVP-пайплайн реплинишмента e_zoo из 7 модулей. Greenfield → готовое решение из 1 монорепо с 2 binaries (`source-adapter` + `etl`).

## Карта модулей

| # | Модуль | Folder | Schema | Cron (Europe/Kyiv) | Endpoints | Tier |
|---|---|---|---|---|---|---|
| 1 | source-adapter | `data_export` | `public.*` (master+facts) | 02:00 | 13 | L |
| 2 | etl-validation | `etl_validation` | `marts.*` | 02:30 (cmd/etl) | 7 | L |
| 3 | data-marts | `data_marts` | (read-side `marts.*`) | — | 4 | M |
| 4 | kpi-calibration | `kpi` | `kpi.*` | 04:00 | 5 | M |
| 5 | forecast-engine | `forecast` | `forecast.*` | 05:00 | 6 | L |
| 6 | order-builder | `orders` | `orders.*` | 06:00 | 5 | M |
| 7 | channel-routing | `channels` | `channels.*` | 06:30 | 6 | M |

## Поток данных

```
ERP клиента → [source-adapter cron 02:00]
            → public.* (16 entities) + snapshot_pointer atomic flip
            → [etl cron 02:30]
            → marts.* (5 mart-таблиц через cross-validation + atomic flip)
            → [kpi cron 04:00]
            → kpi.kpi_snapshots (OSA, OTIF, Stock Days)
            → [forecast cron 05:00]
            → forecast.forecasts + replenishment_plans (status=draft)
            → [admin approve manually]
            → forecast.replenishment_plans (status=approved)
            → [order-builder cron 06:00]
            → orders.purchase_orders (status=ready_to_send)
            → [channel-routing cron 06:30]
            → external ERP via ChannelSender (api_key auth, JSON body)
            → orders.purchase_orders (status=sent)
```

## Метрики реализации

- **Git коммитов:** 80+ (атомарно по фазам)
- **Subagent вызовов:** ~30 (research/design/plan/exec/review per module)
- **Миграции БД:** 9 (0001, 0002, 1001, 1002, 2001, 3001, 4001, 5001 + sequences)
- **Schemas в PG:** 5 (public, marts, kpi, forecast, orders, channels)
- **HTTP endpoints:** 46 публичных
- **Тестов:** 350+ unit + 30+ integration (postgres:18-alpine)
- **Прометеус метрик:** 30+
- **Sentinel errors (EV/KPI/FCT/OB/CH):** 30+

## Ключевые архитектурные паттерны (повторяются во всех модулях)

1. **Feature-based** — `internal/features/{name}/{handler,service,repository,...}`
2. **gocron + advisory lock** — single-process cron с гарантией идемпотентности
3. **Atomic snapshot flip** — single tx UPDATE pointer после успешной записи всех данных
4. **JWT + role middleware** — `admin-cli`, `x-flow-etl`, `it-read`, RequireRole/RequireAnyOf
5. **Pluggable interfaces** — SourceReader (M1), Forecaster (M5), ChannelSender (M7) для будущих расширений
6. **YAML-driven validation** — severity engine (M1) + cross-entity engine (M2)
7. **Prometheus metrics** — единый паттерн `{module}_run_total{status}`, `_duration_seconds`, `_errors_total{reason}`
8. **NDJSON streaming** — для больших ответов (16 entity endpoints + 5 marts)
9. **Cursor pagination** — consistent across all modules
10. **go:embed SQL + golang-migrate** — без auto-apply, makefile targets per module

## Open / отложенные вопросы (требуют ответа от клиента)

| Q-NNN | Тема | Эскалация |
|---|---|---|
| Q-001 (M1) | ERP auth method | ИБ E-Zoo |
| Q-002 (M1) | ERP стек клиента | IT E-Zoo |
| Q-003 (M1) | ERP контракт (REST/SOAP/SFTP) | IT E-Zoo |
| Q-004 (M1) | Объём данных + CDC trigger | IT E-Zoo + продукт |
| Q-011 (M1) | Cold retention timeline (S3 365d) | продукт + IT E-Zoo |
| Q-012 (M1) | CI/Hosting timeline | IT E-Zoo |
| Q-012 (M2) | Bi-temporal recompute | next iter |
| Q-013 (M1→M7) | EDI-профиль | next iter после ответа клиента |
| (M7) | Sample order body schema для client ERP | IT E-Zoo |

## Quality gates (финальные)

- `go build ./...` — OK (оба binaries: source-adapter, etl)
- `go vet ./...` — OK
- `go test -race -count=1 ./...` — все unit-тесты OK
- `golangci-lint run ./...` — 0 issues по всем модулям
- `go test -tags=integration` — 30+ integration tests OK на postgres:18-alpine

## Известные ограничения MVP

- **Нет CI** (Q-012 отложено) — только локальные `make`-команды
- **Нет S3 cold-слоя** (Q-011) — всё в hot PG, retention 30-365d по таблицам
- **Нет Web UI** — только REST/admin-CLI
- **Нет multi-tenant** — один клиент E-Zoo
- **ERP integration = stub** (Q-001..003 ждут ответа) — in-memory backend для разработки/тестов
- **Channel = только `erp_api`** — EDI/1С/CRM interface готов, реализация позже
- **Forecast = moving average** — pluggable interface готов для ML-замены
- **Manual plan approval** — нет workflow для автоматизации
- **Audit middleware coverage** — частичное (только в M1, M2 admin endpoints)
- **Полный E2E тест** через docker-compose с реальным запуском ВСЕХ компонентов отложен

## Что дальше (next iter)

### Технический долг
1. Покрытие audit middleware всех `/admin/*` endpoints (M2-M7)
2. Bi-temporal recompute в ETL (Q-012)
3. Полный E2E тест в CI после готовности GitHub Actions
4. Grafana dashboards + Prometheus alert rules + runbooks (через infra-pipeline)
5. Repository select-методы для оставшихся 14 entity в M1 (сейчас 501 placeholder для 14/16)

### Бизнес-критичное (требует ответа клиента)
1. **ERP integration** — после ответа ИБ/IT E-Zoo:
   - Auth method (Q-001)
   - Контракт (Q-003)
   - Sample request/response для всех endpoints
2. **EDI implementation** — Q-013 после согласования с поставщиками
3. **Объёмы / CDC** — Q-004, замер у E-Zoo IT, переход на CDC если >10M строк/сутки
4. **Cold retention S3/Parquet** — Q-011 после превышения PG-объёмов

### Расширения функциональности
1. ML forecaster (Prophet, LightGBM) — pluggable interface готов
2. Promo lift modeling
3. Multi-location optimization (transfer between locations)
4. Real-time KPI updates (вместо daily snapshots)
5. Multi-channel send (один PO в несколько каналов)
6. Async confirmation webhooks от ERP (status='confirmed_by_erp')
7. Bulk batching POs одного supplier в один EDI message
8. Custom KPI definitions (SLA, fill rate, forecast accuracy)
9. Calibration UI

## Ссылки на отчёты модулей

1. [source-adapter](source-adapter/report/output.md)
2. [etl-validation](etl-validation/report/output.md)
3. [data-marts](data-marts/report/output.md)
4. [kpi-calibration](kpi-calibration/report/output.md)
5. [forecast-engine](forecast-engine/report/output.md)
6. [order-builder](order-builder/report/output.md)
7. [channel-routing](channel-routing/report/output.md)

---

**Pipeline завершён. MVP готов к next iter после получения ответов от E-Zoo IT/ИБ по открытым Q-NNN.**
