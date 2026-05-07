# Code Review Report: etl-validation

**Дата:** 2026-05-07
**Скоуп:** Модуль 2 «X-Flow ETL» (фазы 01–16, все `completed` в `code-plan-status.md`).
**Статус сборки:** `go build ./...` — clean, `go vet ./...` — clean.

---

## Блокеры

Нет.

---

## Серьёзные замечания

### S-1. Pipeline пропускает Extract → Stage → Validate стадии (MVP-симуляция)

**Файл:** `internal/features/etl_validation/service/etl_pipeline.go:140-156`

```go
// 2/3/4: реальный extract+stage+validate был бы тут (Phase 13/14 интеграция).
// Для MVP: пустой Dataset → quality gate проходит автоматически.
dataset := validation.NewDataset()
report := p.engine.Run(dataset)
```

Pipeline всегда стартует с пустым `Dataset` — `Extract`/`Stage`/`Validate` стадии не реализованы.
Это означает:
- `extr.StreamEntity` нигде не вызывается на горячем пути (только из `mart_refresh` через `GetCurrentSnapshot`);
- `report.LinesTotal=0` всегда → quality threshold не работает;
- `loader.Apply` запускает builder-ы поверх пустого `pg_temp.stg_*` → mart-ы будут построены на пустых staging-данных.

В `code-plan-status.md` это отражено как "интеграционный тест pipeline через mock source-adapter — TODO в Validation-стадии",
но критичная логика (Q-005 atomic snapshot read через `?snapshot=<load_id>`, Q-017 full read per snapshot, ADR-005) в коде НЕ реализована.

**Рекомендация:** документировать как явный технический долг в `code-plan-status.md` с пометкой `phase-13-incomplete: extract/stage/validate stub-нуты`. Перед production — обязательная реализация.
Не блокер для review (фазы помечены `completed` с оговорками), но это самое крупное расхождение между design и кодом.

### S-2. JWT для admin-эндпоинтов не реализован — только X-Admin-Secret

**Файлы:**
- `internal/features/etl_validation/router/middleware.go:14`
- `internal/etlapp/app.go:109`
- ADR-022 / `design-integrations.md` §2.2

Дизайн (Q-022/ADR-022, design-integrations.md §2.2) требует **JWT с ролями `admin-cli` и `it-read`**, импорт `internal/middleware/jwt` + `internal/middleware/role` Модуля 1.
Реализован простой `X-Admin-Secret` middleware (как в Модуле 1 для admin-операций). Конфиг-поле называется `AdminJWTSecret`, но фактически используется как shared secret.

**Последствия:**
- Нет ролевого разделения `admin-cli` vs `it-read` (дизайн §2 «Контекст и пользователи» предполагает: IT E-Zoo — read-only через `it-read`, DevOps — full через `admin-cli`).
- `requesterFromCtx` извлекает `user_id`/`sub` из `c.Locals`, но никто их туда не кладёт → `requester` всегда пустой → `audit_access.requester` всегда NULL.

Путь `internal/middleware/jwt`/`role` (по grep — не существует в текущем репозитории; это ожидаемый код Модуля 1, но его реализация в репозитории не обнаружена).

**Рекомендация:** документировать как технический долг (явно отмечено в плане «Audit middleware отложен»). Перед prod — реализовать JWT или формально пересмотреть ADR-022.

### S-3. Audit middleware не реализован — `audit_access` пишется только если route её явно вызовет

**Файлы:**
- `internal/features/etl_validation/router/router.go:13-35` — `Audit fiber.Handler` проброшен в `Middlewares`, но в `etlapp/app.go` НЕ передаётся
- `internal/features/etl_validation/repository/audit_access.go` — `InsertAuditAccess` существует, но никто не вызывает

`router.Middlewares.Audit` — поле есть, но в `etlapp.New` он не настраивается:
```go
router.Register(apiV1, h, router.Middlewares{
    Admin: router.AdminSecretMiddleware(cfg.AdminJWTSecret),
})
```
→ Audit не активирован. ADR (design-go-layers.md §5, design-rest §8 Безопасность) требует «audit_access — пишется на каждый admin-запрос (как в Модуле 1)».

В `code-plan-status.md` фаза 15 явно отмечает: "Audit middleware отложены".

**Рекомендация:** записать как technical-debt issue, но требование к `audit_access` критично для compliance-сценария (IT просмотр истории admin-действий). Перед prod — обязателен.

---

## Незначительные замечания

### N-1. `cmd/etl/main.go` импортирует `internal/logger` — пакет существует?

`cmd/etl/main.go:13` импортирует `"github.com/Kitavrus/e_zoo/internal/logger"` (а не `pkg/logger`, как описано в design-integrations.md §3). Это локальный пакет проекта. По build-clean он есть. Достаточно проверить, что используется ровно тот же slog wrapper, что и в Модуле 1.

### N-2. `metricsRecorder` создаётся, но в `mart_refresh` не передаётся

`MartRefreshService` не получает Metrics → ondemand refresh `mart_supplier_scorecard` не отражается в `etl_run_*` метриках (ADR-009 говорит про общий набор `etl_run_*` для full-run, scorecard в этом списке не упомянут — формально не нарушение, но операционно лучше иметь видимость).

### N-3. `markFailed` использует отдельную транзакцию (`UpdateEtlRunStatus`, не Tx-вариант)

`internal/features/etl_validation/service/etl_pipeline.go:206` — `p.repo.UpdateEtlRunStatus(ctx, runID, patch)` без tx.
Корректно (комментарий в коде это объясняет), но дизайн (`design-go-layers.md` §2.7, design-errors.md §5) подразумевает, что markFailed выполняется ВНЕ tx loader-а, чтобы избежать конфликта с rollback. Реализация совпадает — замечание для подтверждения, не блокер.

### N-4. `MartRefreshService.Refresh` вызывает `markFailed` через приватный метод-заглушку

`internal/features/etl_validation/service/mart_refresh.go:95` — `s.repo.UpdateEtlRunStatus(ctx, runID, ...)` (через приватный helper). По grep есть приватный `markFailed` в этом же файле — OK. Но дублирование кода `markFailed` между `EtlPipeline` и `MartRefreshService` — кандидат на extraction в общий helper.

### N-5. Скрипт `etl_runs_update_status.sql` использует `COALESCE($N, finished_at)` — означает невозможность очистить поле

Если потребуется UPDATE, обнуляющий `finished_at` (например, retry или ручная корректировка), запрос не позволит. Сейчас не используется такой кейс, но семантика стоит явного комментария в SQL-файле.

### N-6. `AssertSentinel` в `admin_etl_runs.go:174` — анти-паттерн

```go
func AssertSentinel() error { return errorspkg.ErrEtlRunNotFound }
```
Комментарий объясняет: «компилятору нужно». На самом деле в файле уже есть импорт `errorspkg` и реальное использование (`MapTriggerRunError`/`MapRetryError` в `mappers/helpers.go`). Эта функция — мёртвый код. Удалить.

### N-7. `internal/etlapp/app.go` имеет `//nolint:contextcheck` для `Fiber Listen`

`etlapp/app.go:130` — `nolint:contextcheck // Fiber не принимает ctx; lifecycle управляется через Shutdown.` Комментарий валиден (Fiber v3 Listen — блокирующий, без ctx), CLAUDE.md разрешает `nolint` с обоснованием. OK.

### N-8. `EtlPipelineConfig` — указатель vs значение

В `etlapp/app.go` создаётся `pipelineCfg` неявно, а в `service.NewEtlPipeline` принимается без чёткой документации, дефолтит ли `LockKey=0` или нет. По коду — есть guard `if cfg.LockKey == 0 { cfg.LockKey = AdvisoryLockKey() }`. OK.

### N-9. `migrate-up-etl` использует ту же БД (`source_adapter`)

`Makefile`: `ETL_DSN ?= postgres://adapter:adapter@localhost:5432/source_adapter?...` — это OK, дизайн (ADR-006) явно говорит «schema marts в той же БД». Просто предостережение для оператора, что миграции 1001/1002 запускаются на том же инстансе, что и `data_export`.

### N-10. Код-плэн «верификация статусов» (CLAUDE.md §6 Executing шаг 7) — все статусы проставлены

Все 16 фаз — `completed` в `code-plan-status.md`. В рамках чек-листа ревью статусы корректны.

### N-11. `extractor` не используется в реальном pipeline (только в `MartRefreshService.Refresh`)

См. S-1. Реальные `StreamEntity` вызовы отсутствуют — extractor покрыт unit-тестами (coverage 83.6%), но в e2e flow не задействован.

---

## Соответствие дизайну (таблица)

| Чек | Дизайн | Реализация | Статус |
|---|---|---|---|
| 1. Структура папок | `design-go-layers.md` §1: handler/service/repository/extractor/transformer/loader/validation/validators/scheduler/models/mappers/router/sqls + constants | Все папки на месте + `metrics/` (Phase 16) | OK |
| 2.1 `Repository` composite interface | EtlRun + RejectLog + Mart + AuditAccess + Staging | Реализовано как методы одного `*Repository`, узкие интерфейсы (`Repo`, `EtlRunsUpdater`, `MartUpserter`) определены в местах потребления | OK (реализация прагматичнее, но семантика идентична) |
| 2.2 `Extractor` interface | `GetCurrentSnapshot`, `StreamEntity` | `service.Extractor`, `extractor.Snapshot{LoadID,...}` | OK |
| 2.3 `ValidationEngine` interface | `Run(ds)Report` | `service.ValidationEngine` | OK |
| 2.4 `Transformer` interface | `Builder.Name/Build` + `Registry` | `transformer.Builder`/`Registry` | OK + `OnDemandOnly()` (полезное расширение) |
| 2.5 `Loader` interface | `Apply(ApplyParams) (BuildSummary, error)` | `loader.Loader.Apply` | OK |
| 2.6 `AdvisoryLock` interface | `TryLock`/`DetectStale` | `repository.TryAdvisoryXactLock` (без `DetectStale` — упрощение, заявлено в Q-025/ADR-025) | partial — `DetectStale` отсутствует (документировано, не блокер) |
| 2.7 `EtlPipeline` interface | `TryStart`/`Run` | `service.EtlPipeline.TryStart`/`runAsync` | OK |
| 3. EV-* sentinels (5) | EV-001..005 | `pkg/errorspkg/errors_etl.go` + `support_codes.go` (35–40) | OK + tests (`errors_etl_test.go`) |
| 3. Reuse `ErrSnapshotNotReady`/`ErrQualityThresholdExceeded` | без изменений в `pkg/errorspkg` | reuse через `errors.Is` (по `extractor` коду) — `ErrSnapshotNotReady` упоминается в `scheduler.go` skip, `ErrQualityThresholdExceeded` — implicit (реализовано через ad-hoc `markFailed` reason='quality_threshold') | partial — `ErrQualityThresholdExceeded` не используется как sentinel в pipeline; вместо неё — markFailed с reason. См. S-1, нерезкий разрыв с design-errors.md §5 |
| 4. SQL through go:embed | все queries в `sqls/queries/embed.go` через `//go:embed *.sql` | + `sqls/migrations/embed.go` | OK |
| 5. Q-001 отдельный binary `cmd/etl` | `cmd/etl/main.go` | существует | OK |
| 5. Q-003 cron 02:30 Europe/Kyiv | env `ETL_CRON_SCHEDULE="30 2 * * *"`, `ETL_CRON_TIMEZONE="Europe/Kyiv"` | docker-compose.yml + config.go дефолты | OK |
| 5. Q-006 schema `marts` | `1001_marts_schema.up.sql` создаёт `CREATE SCHEMA IF NOT EXISTS marts` | OK |
| 5. Q-017 full read per snapshot | full read per snapshot — реализуется на уровне extractor + pipeline | extractor готов, pipeline stub-нут (см. S-1) | partial |
| 5. Q-011 YAML validation engine | `validation/engine_adapter.go` + `etl_validation_rules.yaml` | YAML парсится, builtin checks реализованы (`fk_exists`/`unique_business_key`/etc.) | OK |
| 5. Q-010 manual retry | `EtlRunService.Retry` — проверка `status IN (failed, aborted)` → `ErrCannotRetryEtl` | реализовано | OK |
| 5. Q-013 applicable_rule_id в transformer | `mart_calculation_input_truncate_insert.sql` — UNION с prio=1 (order_rule) prio=2 (supply_spec) + `DISTINCT ON` | реализовано в SQL (соответствует ADR-024) | OK |
| 5. Q-015 quality threshold 1% | `ETL_QUALITY_THRESHOLD=0.01`; pipeline проверяет `failureRate > p.cfg.QualityThreshold` | OK (см. caveat в S-1: при `LinesTotal=0` гейт пропускает всё) |
| 5. Q-016 JWT x-flow-etl | `extractor.HS256/RS256TokenSource` с дефолтом `Role="x-flow-etl"` | OK для исходящих к source-adapter; для admin endpoint — см. S-2 |
| 5. Q-022 admin auth | JWT `admin-cli`/`it-read` | заменено на `X-Admin-Secret` — см. S-2 | partial (документировано в плане как технический долг) |
| 6. Fiber v3 синтаксис | `fiber.Ctx`, `c.Bind()` — N/A здесь (POST с пустым телом) | `c.JSON`, `c.Params`, `c.Query`, `c.Locals` — корректно | OK |
| 6. pgxpool, ctx через слои | handler→service→repo с ctx; pgxpool в Repository | OK; **`runAsync` создаёт `context.Background()` с timeout** — это намеренно (detached от HTTP req), документировано комментарием | OK |
| 6. Errors через WriteJSON | `mappers.MapServiceError` использует `errorspkg.WriteJSON` | OK |
| 8. Atomic flip — builders + UpdateEtlRunStatusTx в одной tx | `loader.Apply` открывает tx, вызывает `Builders[].Build(tx)` + `repo.UpdateEtlRunStatusTx(tx)` + `tx.Commit` | OK — каноничная реализация Q-008 |
| Migrations 1001 + 1002 | schema marts + 5 mart-таблиц + etl_runs/reject_log/audit_access + GRANT mart_reader | присутствуют, CHECK constraints для status/kind/trigger/severity, partitioning by RANGE для demand_history/kpi_daily | OK |
| Constants enum sync (CLAUDE.md §8.1) | EtlRunStatuses/Kinds/Triggers/Severities + reflection-tests | константы определены, валидаторы используют `slices.Contains(constants.X, …)` | OK (DTO Swagger sync — отдельная фаза, не входила в скоуп Phase 15) |
| Configs `etl_validation_rules.yaml` | в `configs/` | OK |
| Makefile targets | `migrate-up-etl`, `migrate-down-etl`, `migrate-create-etl`, `migrate-version-etl`, `test-integration-etl`, `build-etl`, `run-etl`, `docker-build-etl` | OK |
| docker-compose etl service | image `e_zoo/etl:dev`, ETL_* env vars, depends_on postgres+source-adapter, port 8081 | OK (Dockerfile.etl multistage builder/runner) |
| pkg/errorspkg | новые `errors_etl.go` + `support_codes.go` (EV-001..005) + tests | OK |

---

## Документированные ограничения (НЕ блокеры)

Все нижеперечисленные технические долги явно зафиксированы в `code-plan-status.md` (фазы 13/14/15/16):

1. **Phase 13 — Extract/Stage/Validate stub.** Pipeline вызывает `engine.Run(NewDataset())` без реального скачивания данных через `extractor.StreamEntity`. Loader строит mart-ы поверх пустых `stg_*`. Заявлено как «интеграционный тест pipeline через mock source-adapter — TODO в Validation-стадии».
2. **Phase 14 — integration concurrency test для scheduler отложен** (нужен полный pipeline mock).
3. **Phase 15 — Handler integration tests + Audit middleware отложены.** Audit middleware (`router.Middlewares.Audit`) есть как точка расширения, но в `etlapp.New` не передаётся → `audit_access` не пишется на admin-запросы.
4. **Phase 15 — JWT middleware заменён на `X-Admin-Secret`.** ADR-022 формально требует `admin-cli`/`it-read` JWT roles; реализован shared-secret.
5. **Phase 16 — Grafana JSON, alert rules, runbook, CLAUDE.md §8 EV-codes — отложены в инфра-задачу.**
6. **Q-025 / ADR-025 (Stale ETL run timeout)** — `DetectStale` метод advisory-lock не реализован; есть `cleanup_failed_runs.sql` query, но cron-job на её регулярный вызов отсутствует.
7. **DTO enum sync-тесты (CLAUDE.md §8.1)** — `etl_run_swag_test.go` / `reject_log_swag_test.go` есть, но это только DTO-side; кросс-проверка с `constants.EtlRunStatuses` через reflection — отсутствует (минор, не блокер).

Все эти пункты — **не препятствие для перехода в Validation**, но обязательны к закрытию перед prod-деплоем.

---

## Дополнительные положительные находки

1. **Loader atomic flip** реализован эталонно: `BeginTx → CreateStagingTables(tx) → Builders[].Build(tx) → UpdateEtlRunStatusTx(tx) → Commit`, с `defer tx.Rollback` (no-op после Commit). Точно соответствует ADR-005/Q-008.
2. **Узкие интерфейсы в местах потребления** (`loader.EtlRunsUpdater`, `loader.PoolBeginner`, `transformer.MartUpserter`, `service.Repo`) — образцовый Go-паттерн, упрощает unit-тесты.
3. **Detached context в `runAsync`** — корректное решение для long-running pipeline, не отменяемого 202-ответом handler-а.
4. **Recover panic в `runAsync`** — ловит и помечает run как failed с reason='panic', корректно записывает метрику.
5. **Coverage** — большинство фаз ≥83% по комментариям в `code-plan-status.md` (transformer 93.1%, loader 94.1%, validators 100%).
6. **`SchedulerSkipMetrics`** правильно различает `already_running` / `snapshot_not_ready` / `error` reasons — соответствует ADR-009.
7. **EV sentinel test (errors_etl_test.go)** — table-driven с проверкой code/HTTP/SupportMessage по 5 элементам.

---

## Итог: APPROVED (с оговорками)

Реализация соответствует одобренному дизайну на уровне архитектуры, интерфейсов, SQL-контракта,
структуры папок, atomic flip, sentinel errors и интеграционного скелета.

5 серьёзных оговорок (S-1..S-3 и партиальные пункты в таблице) — все явно зафиксированы как технический долг
в `code-plan-status.md` (фазы 13/14/15/16 с пометками "отложено в Validation/инфра-задачу").

Перед prod-деплоем обязательно закрыть пункты 1–5 раздела «Документированные ограничения».
Для перехода в стадию Validation — **готов**.
