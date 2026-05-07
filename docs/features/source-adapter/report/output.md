# Pipeline Report: source-adapter
**Дата:** 2026-05-07
**Профиль:** Бизнес-фича
**Scope:** Модуль 1 «Адаптер источников» из MVP-пайплайна реплинишмента e_zoo

## Что сделано

Запрошен полный pipeline (Research → Spec Interview → Design → Plan → Executing → Code Review → Validation → Report) для greenfield-сервиса «Адаптер источников» — единственного PG18-источника правды для X-Flow ETL и пользовательских интеграций. На входе репозиторий был пуст (только `docs/`); на выходе — полностью собранный Go-сервис на Fiber v3 + pgx + golang-migrate, реализующий 16 фаз code-plan, прошедший code review (2 блокера закрыты) и validation (2 critical bug закрыты). Module path: `github.com/Kitavrus/e_zoo`. Feature package: `internal/features/data_export/`. CLI binary: `cmd/source-adapter/main.go`.

## Артефакты pipeline (по стадиям)

| Стадия | Файлы | Итог |
|---|---|---|
| Research | [research/output.md](../research/output.md) | greenfield (нет кода), 16 OQ |
| Spec Interview | [spec-interview/output.md](../spec-interview/output.md) | 16 Q-NNN зафиксировано |
| Design | 13 design-*.md (overview + 11 артефактов + adr-crosswalk) + ADR-001..016 + ADR-100..103 | APPROVED после re-review |
| Swimlane | [design-swimlane.html](../design-swimlane.html) | 5×5 grid, 16 FLOWS |
| Plan | [code-plan.md](../code-plan.md) + [code-plan-status.md](../code-plan-status.md) + 16 phase-файлов | 16 фаз ≤ лимита |
| Executing | весь Go-код в `cmd/`, `internal/`, `pkg/`, `migrations/`, `configs/` | 16/16 done, 16 атомарных фазных коммитов |
| Code Review | [reviewer/output.md](../reviewer/output.md) | 2 блокера → закрыты в `e2090ef` |
| Validation | [validation/output.md](../validation/output.md) | 2 critical bug → закрыты в `6954b69` |

## Ключевые архитектурные решения (топ-7 ADR)

- **ADR-005:** Cron `0 2 * * *` Europe/Kyiv (configurable через `SOURCE_ADAPTER_CRON_SCHEDULE` / `SOURCE_ADAPTER_TZ`).
- **ADR-006:** Severity-движок + 7 стартовых правил YAML (`negative_qty`, `future_event_time`, `duplicate_product_in_load`, `missing_required_field`, `negative_stock_balance`, `orphan_fk` soft, `stale_event_time` soft).
- **ADR-007:** Retry max 3, exponential backoff cap 30s, jitter 10% (для VPN flapping).
- **ADR-008:** `snapshot_pointer` single-row (id=1) + atomic flip `current_load_id` в одной транзакции вместе с `loads.status='committed'`.
- **ADR-009:** Local FS exports (`/var/exports/{id}.{ext}`, retention 24h, cleanup-cron внутри сервиса) — без S3 на MVP.
- **ADR-014:** Audit retention 90d, аудит пишется только для `/admin/*`.
- **ADR-015:** `SOURCE_ADAPTER_STALE_LOAD_TIMEOUT` default `1h` — записи `loads.status='running'` старше 1ч помечаются `aborted` при следующем cron-tick.

Полный crosswalk Q-NNN ↔ ADR-NNN (1:1) — в [design-adr.md](../design-adr.md). Мета-ADR: ADR-100 (стек Go/Fiber v3/pgxpool/go:embed), ADR-101 (JWT), ADR-102 (atomic flip), ADR-103 (multi-tenant — отложен).

## Что реализовано (фазы 1–16)

| # | Фаза | Аннотация |
|---|---|---|
| 01 | Bootstrap | `go.mod` (`github.com/Kitavrus/e_zoo`), `cmd/source-adapter`, `internal/app`, `pkg/errorspkg`, `docker-compose.yml`, `Makefile`. |
| 02 | JWT middleware | HS256/RS256 + role gating (`x-flow-etl`, `admin-cli`, `it-read`); 13+ unit-тестов. |
| 03 | Migrations master | 17 таблиц (мастер + service); integration-тесты на dockertest. |
| 04 | Migrations facts | 4 partitioned-by-`event_date` фактов + initial 4 месячные RANGE-партиции. |
| 05 | Models / DTO | Доменные модели и DTO для 16 сущностей + admin endpoints; cursor + dto-валидаторы покрыты тестами. |
| 06 | SQL queries | 32 SQL-файла + `embed.go`, выровнены под фактические migrations 0001/0002. |
| 07 | Validators + Engine + YAML | Формат-валидаторы + severity-engine + 7 правил YAML + 3 sentinel. |
| 08 | Repository | pgx Repository + 15 integration-тестов на dockertest postgres:18-alpine + 6 sentinel. |
| 09 | SourceReader stub | Generic `SourceReader` interface (16 entities) + `ErpEZooReader` in-memory stub + 16 fixtures + 7 unit-тестов. |
| 10 | Loader service | Pipeline master → facts → flip + 1% quality threshold; `LoaderAPI` interface; 7 unit-тестов на mock SourceReader. |
| 11 | Snapshot + Audit | `snapshot.Service` (atomic flip) + `audit.Writer` middleware (только `/admin/*`); 4+4 unit-теста. |
| 12 | Scheduler + admin | `gocron` WithSingletonMode + advisory lock + partitions pre-step + `POST /admin/loads`, `POST /admin/loads/{id}/retry`, `GET /admin/loads/{id}`, `GET /admin/reject-log`. |
| 13 | Read handlers | `/v1/{entity}` + `/v1/snapshots/current` + `/v1/healthz` + NDJSON streaming + ETag/`If-None-Match`. |
| 14 | Exports storage | Local FS + `POST /v1/exports` + `GET /v1/exports/{id}` + cleanup-cron (retention 24h). |
| 15 | Router + DI | `internal/features/data_export/router` + `internal/routers` + полный DI в `app.New` (pool→repo→snapshot→loader→scheduler→exports→audit→handlers→router). |
| 16 | Metrics + observability | Prometheus (10 метрик) + `HTTPMetricsMiddleware` + `AccessLogMiddleware` (slog) + `/metrics`; 5 unit-тестов. |

## Метрики прогона

- Уникальных subagent-вызовов: ~15 (research, spec-interview, design, design-review, design-rereview, plan, 16× executing, reviewer, fix-blockers, validation, fix-validation, report).
- Git коммитов всего на ветке: 70; коммитов фазы реализации фичи: **18** (16 фазных `feat(...)` + `e2090ef` fix code-review + `6954b69` fix validation).
- Файлов design: **14** (12 `design-*.md` + `design-swimlane.html` + `design-adr.md`).
- Файлов code-plan: **18** (`code-plan.md` + `code-plan-status.md` + 16 фазных).
- Go-файлов в репозитории: **97** (по `git ls-files '*.go'`).
- Тестовых Go-файлов: **27** (`*_test.go`); по логам фаз — ~120 unit + 15 integration на dockertest postgres:18-alpine.
- Severity-rules: **7** стартовых (configs/validation_rules.yaml).
- Prometheus метрик: **10**.
- ADR: **20** (16 закрывающих Q-NNN + 4 мета).

## Открытые вопросы / Q-NNN со статусом «Отложено»

| Q-NNN | Тема | ADR | Эскалация |
|---|---|---|---|
| Q-001 | ERP auth method (OAuth2 cc / mTLS / API-key) | ADR-001 | ИБ E-Zoo (CISO) |
| Q-002 | ERP стек клиента (1С / SAP / кастом) | ADR-002 | IT E-Zoo |
| Q-004 | Объём данных + CDC trigger (>10M строк/сутки) | ADR-004 | IT E-Zoo + продукт E-Zoo |
| Q-011 | Cold retention timeline (S3/Parquet 365d) | ADR-011 | продукт + IT E-Zoo |
| Q-012 | CI/Hosting timeline (GitLab CI / GH Actions; k8s / VM) | ADR-012 | IT E-Zoo |
| Q-013 | EDI-профиль для маршрутизации заказов | ADR-013 | передан в Модуль 7 |
| ADR-103 | Multi-tenant архитектура | meta | продукт E-Zoo (post-MVP) |

Остальные 10 Q-NNN закрыты ADR-ами как «Принято» (Q-003, Q-005…Q-010, Q-014, Q-015, Q-016).

## Известные ограничения (MVP)

- Repository select-методы реализованы только для `products` + `receipt_line`. Остальные 14 entity → `501 Not Implemented` (документировано в [code-plan-status.md](../code-plan-status.md), фаза 13).
- ERP reader = in-memory stub (`erp_e_zoo_reader.go`) — Q-001..Q-003 blocked. Реальная интеграция требует ответа от E-Zoo IT/ИБ.
- CI отсутствует (Q-012 — отложено).
- S3 cold-слой не введён (Q-011 — отложено).
- Web UI отсутствует (non-goal MVP; admin-CLI + Prometheus/Grafana).
- Multi-tenant отсутствует (non-goal MVP; ADR-103 отложен).
- CDC отсутствует (non-goal MVP; включается при Q-004 → >10M строк/сутки).
- Loader/scheduler instrumentation: метрики **зарегистрированы**, но инкременты в loader-pipeline и scheduler-tick подключаются по мере, post-MVP (см. фаза 16 note).

## Найденные и исправленные баги в validation

- **Issue #5 (HIGH, real bug):** Loader entity-order нарушал FK `products_category_id_fkey` (вставка `products` до `category`). **Исправлено** в `6954b69`: правильный порядок `category → products → product_barcodes → location → store_assortment → … → facts`.
- **Issue #6 (MEDIUM, real bug):** `POST /admin/loads` не возвращал `409 Conflict` при concurrent. **Исправлено** в `6954b69`: `TryTrigger` через PG advisory lock + добавлен unit-тест.
- **Issue #1 (LOW):** `docker compose up -d postgres` падал из-за PGDATA. **Исправлено** в `6954b69`: правильный `PGDATA` mount в `docker-compose.yml`.
- **Issue #2 (LOW):** `make migrate-up` использовал несуществующий тег `migrate/migrate:v4`. **Исправлено** в `6954b69`: pinned tag.
- **Issue #3 (LOW):** Порт 8080 конфликтовал. **Исправлено** в `6954b69`: настройка в `.env.example`.
- **Issue #4 (LOW):** Без `ERP_BASE_URL` сервис стартовал с `noopTrigger` молча. **Исправлено** в `6954b69`: WARN в логе при старте.

Code-review блокеры (закрыты в `e2090ef` до validation): handler возвращал 501 без указания entity; недостающий `ctx` в нескольких pgx-вызовах; одна Prometheus-метрика регистрировалась дважды.

## Quality gates (финальные)

- `go build ./...` — **OK**
- `go test ./internal/... ./pkg/... -short -race` — **OK** (~120 unit-тестов)
- `go test -tags=integration ./internal/features/data_export/repository/...` — **OK** (15 integration-тестов через dockertest postgres:18-alpine)
- `golangci-lint run` — **0 issues**
- design-review re-review — **APPROVED** (1 блокер + 7 серьёзных + 4 незначительных закрыты)
- code-review — **APPROVED после fix** (2 блокера закрыты в `e2090ef`)
- validation — **PASSED после fix** (2 critical bug + 4 housekeeping закрыты в `6954b69`)

## Что дальше (next iter после MVP)

- Закрыть Q-001..Q-003 после ответа E-Zoo IT/ИБ → подключить реальный `erp_e_zoo_reader` вместо stub-а (выбрать `SourceAuth` impl: `bearerAuth` / `mtlsAuth` / `apiKeyAuth`).
- Дописать repository select-методы для оставшихся 14 entity (сейчас отдают `501`).
- Добавить CI (GitHub Actions / GitLab CI) — Q-012.
- Ввести cold-слой S3/Parquet retention 365d — Q-011.
- Подключить CDC, если объём `receipt_line` превысит 10M строк/сутки — Q-004.
- Расширить severity-rules + e2e интеграционные тесты по cron-выгрузке (включая stale-load detection и atomic flip rollback).
- Подключить инкременты loader/scheduler метрик (фаза 16 note).
- Подключить Модуль 2 (X-Flow ETL) к API адаптера через контракт витрин (`docs/tmp/data-marts/contract-2026-05-06.md`).
- Передать Q-013 в research/spec Модуля 7 (EDI-профиль для маршрутизации заказов).

## Ссылки на артефакты

- [Research](../research/output.md)
- [Spec](../spec-interview/output.md)
- [Design overview](../design.md)
- [Design ADR](../design-adr.md)
- [Design C4](../design-c4.md)
- [Design Dataflow](../design-dataflow.md)
- [Design DI](../design-di.md)
- [Design Errors](../design-errors.md)
- [Design Go-layers](../design-go-layers.md)
- [Design Infrastructure](../design-infrastructure.md)
- [Design Integrations](../design-integrations.md)
- [Design Sequence Diagrams](../design-sequence-diagrams.md)
- [Design SQL](../design-sql.md)
- [Design Tests](../design-tests.md)
- [Design Review Report](../design-review-report.md)
- [Swimlane HTML](../design-swimlane.html)
- [Code Plan](../code-plan.md)
- [Code Plan Status](../code-plan-status.md)
- [Code Review](../reviewer/output.md)
- [Validation](../validation/output.md)

## Ответ оркестратору

- **Финальный отчёт:** `/Users/igorpotema/mycode/e_zoo/docs/features/source-adapter/report/output.md`
- **Pipeline завершён:** Research → Spec → Design (APPROVED) → Plan → Executing (16/16) → Code Review (APPROVED после fix) → Validation (PASSED после fix) → Report.
- **Готовность:** MVP «Адаптер источников» собран, протестирован, инфраструктурные блокеры устранены. Можно подключать Модуль 2 (X-Flow ETL).
- **Открытые внешние блокеры:** Q-001/Q-002/Q-004/Q-011/Q-012 ждут ответа от E-Zoo IT/ИБ/продукта; Q-013 передан в Модуль 7.
