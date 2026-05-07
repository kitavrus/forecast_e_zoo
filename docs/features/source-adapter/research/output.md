# Research: source-adapter

## Module
- Имя модуля: отсутствует. В репозитории `/Users/igorpotema/mycode/e_zoo/` нет `go.mod`, нет `cmd/`, нет `internal/`, нет `pkg/`. Стек Go 1.26 / Fiber v3 / pgx / golang-migrate / dockertest заявлен только в опорных документах (см. `docs/tmp/replenishment/spec-2026-05-06.md` §4.7), но реального кода нет.

## Что уже есть в коде

### Backend
- Файлы: отсутствуют. Найдено только:
  - `docs/` — спецификации, диаграммы, draft-plan
  - `.claude/` — конфигурация агентов
  - `.gitignore` — заточен под Go (binaries, coverage, `go.work`, `.env`)
  - `.idea/` — JetBrains, в т.ч. `amplicode-jpa.xml`, `e_zoo.iml` (заготовка под Java/IDE-проект, исходников нет)
- Сервисы: отсутствуют.
- Репозитории: отсутствуют.
- Маршруты: отсутствуют (нет ни `internal/routers/routers.go`, ни `internal/app.go`).
- Схема БД: отсутствуют миграции. Папки `migrations/` нет.
- Внешние интеграции: отсутствуют.
- Error flow: отсутствует. Пакет `pkg/errorspkg` упоминается только текстуально в `docs/tmp/replenishment/spec-2026-05-06.md` §7.2 (`pkg/errorspkg.ErrorResponseJSON` с полями `code`, `message`, `supportMessage`, `traceId`).
- Валидация: отсутствует. В `docs/tmp/data-export/spec-2026-05-05.md` §4.3, ADR-006 описана будущая severity-валидация на YAML (`validation_rules.yaml`).
- Scheduler/cron: отсутствует. В spec-data-export ADR-010 заложен «Go-cron внутри процесса» + `POST /admin/loads`.

### Frontend
- Папка `frontend/` отсутствует. `package.json` нет нигде.

### Contracts
- `.env.example` отсутствует.
- OpenAPI отсутствует (упомянут как `api/openapi/v1.yaml` в spec-data-export §«Critical files (планируемые)»).
- Документированный (планируемый) набор маршрутов взят из `docs/tmp/data-export/spec-2026-05-05.md` §8:
  - `GET /v1/healthz`
  - `GET /v1/snapshots`, `GET /v1/snapshots/current`
  - `GET /v1/products`, `/v1/product_barcodes`, `/v1/category`, `/v1/location`, `/v1/supplier`
  - `GET /v1/store_assortment`, `/v1/store_assortment/lifecycle_events`
  - `GET /v1/master_change_log`, `/v1/supplier_stock_snapshot`
  - `GET /v1/supply_spec`, `/v1/promo`, `/v1/supply_plan`, `/v1/order_rule`
  - `POST /v1/exports`, `GET /v1/exports/{id}`
  - `POST /admin/loads`, `POST /admin/loads/{id}/retry`, `GET /admin/loads/{id}`
  - `GET /admin/reject-log`
- Заголовки: `X-Snapshot-Id`, `X-Load-Id`, `ETag`, `Cache-Control: private, max-age=86400`.
- Content-negotiation: `application/x-ndjson` inline (мелкие сущности) и Parquet через signed URL S3 (большие).
- Rate limiting: упомянут только косвенно (rate-limit poll-а) для replenishment, в адаптере нет.
- Error responses: `pkg/errorspkg.ErrorResponseJSON` (планируется).

### Infrastructure
- `docker-compose*.yml`: отсутствует.
- `Dockerfile`: отсутствует.
- `Makefile`: отсутствует.
- `.gitlab-ci.yml` / `.github/workflows/`: отсутствуют.
- `nginx.conf` / `Caddyfile` / `systemd`: отсутствуют.
- Secrets management: не выбран.
- Внешние сервисы: PG18 — заявлен в spec; S3 — для cold-retention Parquet (ADR-009); Prometheus/Grafana — для метрик. Ничего из этого не подключено в репо.
- Логирование/мониторинг: только в спецификации (Prometheus metrics, structured logs).

### Tests
- Unit/integration тесты: отсутствуют (нет `*_test.go`).
- Base suite/dockertest: отсутствует. dockertest/v3 заявлен в `docs/tmp/replenishment/spec-2026-05-06.md` §4.7.
- Моки: отсутствуют.
- Frontend-тесты: не применимо.
- CI: отсутствует.

## Паттерны в коде
Паттерны в коде отсутствуют — кодовой базы нет. Обязательные ориентиры (из спецификаций, не из кода):
- Структура `internal/features/<feature>/{handler,service,repository,models,router,sqls}` (ссылка: `docs/tmp/replenishment/spec-2026-05-06.md` §4.7).
- Планируемые «Critical files» для адаптера (ссылка: `docs/tmp/data-export/spec-2026-05-05.md`):
  - `internal/features/data_export/router/router.go`
  - `internal/features/data_export/loader/` + `loader/erp_e_zoo_reader.go`
  - `internal/features/data_export/validation/`
  - `internal/features/data_export/storage/migrations/`
  - `internal/features/data_export/exports/`
  - `configs/validation_rules.yaml`
  - `api/openapi/v1.yaml`

## Чего НЕТ (потребуется создать)
- `go.mod` + module path.
- `cmd/server/main.go` (graceful shutdown, env loading).
- `internal/app.go` (Fiber config, middleware, DI).
- `internal/routers/routers.go`.
- Все слои фичи `data_export` / `source-adapter`: handler, service, repository, models, router, sqls, validators.
- `internal/middleware/` — вообще ни одного middleware.
- `pkg/errorspkg` — sentinel-ошибки, `ErrorResponseJSON`.
- `pkg/utils/` и любые внешние интеграции (HTTP client, S3, Prometheus).
- `mappers/` (`MapServiceError`).
- Конфиг: `config/db/`, `config/app/`.
- Миграции `migrations/` под все таблицы из §5 spec-data-export:
  - мастер: `products`, `product_barcodes`, `category`, `location`, `store_assortment`, `store_assortment_lifecycle_events`, `supplier`, `supply_spec`, `promo`, `order_rule`, `supply_plan`, `master_change_log`
  - факты: `receipt_line`, `location_stock_snapshot`, `stock_movement`, `supplier_stock_snapshot`
  - служебные: `loads`, `snapshot_pointer`, `reject_log`, `entity_checkpoint`, `audit_access`
- HTTP-клиент с retry/timeout/auth.
- Scheduler (Go-cron внутрипроцессный по ADR-010).
- Severity-движок валидации + `validation_rules.yaml`.
- S3-writer для Parquet (cold retention 365d).
- OpenAPI v1.
- Dockerfile, docker-compose, CI.
- Любые тесты + dockertest base suite.

## Зависимости (что затронет изменение)
- Контракт витрин (`docs/tmp/data-marts/contract-2026-05-06.md`): X-Flow ETL потребляет именно сущности из §1 этого документа: `products`, `product_barcodes`, `category`, `location`, `store_assortment`, `supplier`, `supply_spec`, `promo`, `order_rule`, `supply_plan`, `receipt_line`, `location_stock_snapshot`, `stock_movement`, `supplier_stock_snapshot`, `master_change_log`, `store_assortment_lifecycle_events` — это формирует обязательный набор endpoints/таблиц адаптера.
- Replenishment (`docs/tmp/replenishment/spec-2026-05-06.md` §4.5): зависит только от витрин (`mart_*`), но опирается на свежесть, которая упирается в успешные load-ы адаптера.
- ERP клиента (`docs/tmp/data-export/erp-integration-requirements.md`): три варианта контракта (REST API / SOAP / SFTP), три варианта auth (OAuth2 cc / mTLS / API-key + IP allowlist), VPN site-to-site. Реальный выбор не сделан — блокер OQ-1, OQ-3.
- При появлении кода затрагивается весь репозиторий (создание скелета Go-сервиса).

## Граничные случаи из кода
Не применимо — кода нет. Граничные случаи, зафиксированные в спецификациях (для последующего этапа):
- Дубликат `product_id` внутри одного `load` → critical fail (spec-data-export §4.5).
- Отрицательные остатки / qty в чеке → critical (§4.3).
- `event_time` в будущем (>now+15min) → critical (§4.3).
- `lines_failed / lines_total > 5%` (по replenishment) → run failed.
- VPN flapping → длительные ретраи, нужен backoff cap (§11 риски).
- ERP REST не вытянет >10M `receipt_line/сутки` → риск, потребуется CDC раньше плана.

## PostgreSQL 18 специфика
- ADR-002: материализация в PG18 (а не stream-through).
- spec-replenishment §4.7: партиционирование `forecasts` / `replenishment_plans` по `run_id` с virtual generated `run_month` — для адаптера не применяется напрямую, но указывает на использование PG18 partitioning.
- В адаптере планируются partitioned tables по `event_date` для фактов (упоминается через retention 30d hot PG, ADR-009) — конкретная DDL-стратегия в коде отсутствует.
- Использование advisory lock в PG для исключения параллельного запуска cron (§4.1).
- Никакого Redis/Kafka/RabbitMQ — упор на PG18 как единственное хранилище очередей/состояния (см. spec-replenishment §4.5).

## Адаптер источников: что уже есть, чего нет
- Паттерн адаптера / порта / интерфейса для внешних источников: **в коде отсутствует**. Кодовой базы нет.
- В спецификации `docs/tmp/data-export/spec-2026-05-05.md` интерфейс заявлен как `SourceReader` (Non-goals: «Multi-source — интерфейс `SourceReader` оставляем, реализация одна») и «Critical files»: `internal/features/data_export/loader/` + `loader/erp_e_zoo_reader.go`. Это план, не реализация.
- DTO/мапперы: упомянуты только в `docs/features/data-export/draft-plan.md` («должны быть ДТОшки, должны быть мапперы»). Кода/интерфейсов нет.
- Severity-движок валидации (`validation_rules.yaml`, ADR-006): отсутствует.
- Будет создаваться **с нуля**.

## Frontend: применимо или нет
- **Не применимо для MVP.**
- Обоснование: (1) папки `frontend/` нет; (2) `docs/tmp/data-export/spec-2026-05-05.md` §3 «Users & use cases» прямо указывает: для DevOps — Prometheus/Grafana + admin-CLI; для X-Flow — OpenAPI v1 + admin-API; для IT E-Zoo — «read-only dashboard (отсроченно, не MVP)»; собственный ops UI / business dashboard в Non-goals; (3) `docs/tmp/replenishment/spec-2026-05-06.md` §6.1 — Web UI на Vue отложен на v2.
- Поверхность управления адаптером в MVP: только REST `POST /admin/loads`, `POST /admin/loads/{id}/retry`, `GET /admin/loads/{id}`, `GET /admin/reject-log`.

## Открытые вопросы для Spec Interview
Из `docs/tmp/data-export/spec-2026-05-05.md` §11 и `docs/tmp/data-export/erp-integration-requirements.md`:
1. **OQ-1 / Auth.** Какой метод аутентификации к ERP API: OAuth2 client_credentials (предпочтительно), mTLS или API-key + IP allowlist? Нужно решение ИБ E-Zoo до старта реализации.
2. **OQ-3 / ERP-стек.** Какой именно ERP у клиента (1С версия?, SAP, кастом) и какие endpoints доступны?
3. **Контракт.** Вариант A (REST с курсорной пагинацией, ETag/If-Modified-Since, JSON/JSONL, gzip, HTTPS), вариант B (SOAP) или C (SFTP CSV/Parquet/JSONL + manifest + atomic rename)? Что E-Zoo реально готова реализовать?
4. **OQ-2 / Объёмы.** Реальный объём (SKU, locations, receipt_line/сутки) — потенциальное превышение >10M строк/сутки означает невозможность pull-режима.
5. **Расписание.** Конкретное время и TZ ежедневной выгрузки (по умолчанию «раз в сутки», но без дефолтного слота).
6. **Идемпотентность.** Подтвердить семантику `POST /admin/loads` (force=true), advisory lock в PG, поведение на параллельный запуск.
7. **Ретраи.** Backoff cap на VPN flapping; ретрай по сущности vs по всему load-у; политика для `entity_checkpoint`.
8. **Размер выгрузки.** Threshold для перехода между NDJSON inline и Parquet async (в §8 указано `< 50 MB` inline — подтвердить).
9. **Severity-rules.** Стартовая политика (отрицательные остатки = critical) — кто owner и как меняем (через PR на YAML).
10. **Retention.** Подтвердить 30d hot PG + 365d cold S3; куда писать Parquet (provider, bucket, путь, naming).
11. **Snapshot-семантика.** Что значит «committed snapshot» при ошибке частичной загрузки: атомарный flip `snapshot_pointer.current_load_id` после успеха всех сущностей, или per-entity?
12. **OQ-11 / supplier_stock.** Какие поставщики дают видимость остатков и как (EDI INVRPT / REST / Excel / нет)?
13. **OQ-4 / EDI-профиль.** Не блокирует адаптер, но влияет на маршрутизацию заказов (вне модуля 1).
14. **Audit для IT E-Zoo.** Какой объём данных писать в `audit_access` и кто читает (отсроченный dashboard)?
15. **Module path.** Какой Go module path использовать (например `gitlab.com/e-zoo/replenishment` или `github.com/...`)?
16. **CI/Hosting.** Где CI (GitLab CI / GitHub Actions) и куда деплоим (X-Flow cloud, k8s или bare VM)?

## Важно
Документ содержит **только факты**: в репозитории нет ни одной строки кода, нет миграций, нет инфраструктуры. Все «как должно быть» — цитаты из спецификаций (`docs/`), а не из существующего кода. Любая реализация source-adapter будет создавать проект с нуля.
