# Code Review Report: source-adapter

**Дата:** 2026-05-07
**Ревьюер:** code-reviewer agent
**Скоуп:** Go-код source-adapter против design-пакета `docs/features/source-adapter/design*.md`.
**Контракт стадии:** полнота/консистентность дизайна уже подтверждена на Stage 5 — здесь проверяем соответствие кода дизайну.

---

## Блокеры

1. **`NotImplementedHandlers` отвечают 500, а не 501.**
   `internal/features/data_export/handler/not_implemented.go:21` использует `errorspkg.ErrInternal.WithMessage(...)`. У `ErrInternal.HTTP = 500`, поэтому 14 заглушек по факту возвращают `500 internal_error`, а не 501. Комментарий в файле явно говорит «501», status.md и design тоже подразумевают 501 для not-implemented. Это вводит в заблуждение клиентов и системы мониторинга (alert на 5xx vs 501 = ожидаемая фича пост-MVP). Нужно либо ввести `ErrNotImplemented` (HTTP 501) в `pkg/errorspkg`, либо отдавать `c.SendStatus(fiber.StatusNotImplemented)` напрямую. Это не «документированное ограничение» — это баг в реализации.

2. **`mappers/` есть как пакет ERP↔domain, но «error mapping в отдельном пакете `mappers/`» из чек-листа задачи отсутствует.**
   В `internal/features/data_export/mappers/{master.go,facts.go}` лежит lossless DTO-маппинг (это корректно по design-go-layers §1). Но мэппинг ошибок (sentinel → HTTP) лежит непосредственно внутри хендлеров через `errorspkg.WriteJSON(c, errorspkg.ErrXXX.Wrap(err))` (см. `handler/exports.go:36`, `handler/products.go`, `handler/admin_loads.go` — все 50+ обращений к `errorspkg`). Это РАСХОЖДЕНИЕ с требованием задачи «Error mapping в отдельном пакете `mappers/`, НЕ в handler/helpers.go». Сам design-errors.md этого требования не накладывает (он описывает `WriteJSON` как helper и `FiberErrorHandler` как catch-all), поэтому формально код соответствует design-errors.md. Помечаю как блокер именно по чек-листу ревью; если оркестратор считает это требованием design-go-layers — нужно вынести error-mapping helpers в отдельный sub-пакет.

---

## Серьёзные замечания

1. **`context.Background()` в продакшен-коде вместо проброса `ctx` из вызова.**
   Найдено 5 кейсов в продакшен-коде:
   - `handler/admin_loads.go:66, 114` — `h.trigger.TriggerOnce(context.Background())` теряет request-context (deadline, traceID, отмена). Должно использовать `c.Context()`.
   - `scheduler/scheduler.go:107` (`runTick`) и `scheduler/exports_cleanup.go:25` — gocron не пробрасывает ctx, так что это вынужденная мера, но желательно завести `appCtx` на уровне `App` и пробрасывать его в scheduler.
   - `exports/service.go:70` — фоновая запись экспорта через `context.Background()` — приемлемо, но желательно `context.WithoutCancel(parentCtx)` (Go 1.21+) или явно прокомментировать причину.

2. **Sentinel `ErrInternal` не имеет ни одного теста.**
   По матрице `design-tests.md §6` `ErrInternal` должен покрываться unit-тестом catch-all-handler-а в `internal/middleware/error_handler_test.go`. Файла нет. `ErrInternal` обнаруживается в `pkg/errorspkg/errors_test.go` только через `TestErrInternal`-подобные кейсы по `Code/HTTP`, но catch-all поведение `WriteJSON` для произвольной ошибки (которая мапится на 500 internal) — не покрыто отдельно.

3. **Loader/Scheduler instrumentation метрики НЕ инкрементируются.**
   `grep metrics.` по `loader/*.go` и `scheduler/*.go` пуст. status.md фиксирует это как замечание — соглашусь, но оставлю в «Серьёзных», т.к. без `load_success_total` / `load_failed_total{reason}` / `lines_total` / `quality_threshold_exceeded_total` runbook становится бесполезен на проде. Метрики зарегистрированы (`internal/observability/metrics.go`), но не вызываются.

4. **Repository select-методы покрывают только `products` + `receipt_line`.**
   Это явно зафиксировано в `code-plan-status.md` (Phase 13 done note + Phase 08 note) и закрыто 14 заглушками `NotImplementedHandlers`. По правилам ревью — НЕ блокер. Фиксирую как серьёзное напоминание для пост-MVP, потому что 14 entity без репозитория = функционально половина API не работает.

5. **ERP HTTP-клиент отсутствует.**
   В `loader/` есть только `erp_e_zoo_reader.go` — in-memory reader из fixtures. Реальный `net/http` клиент с retry/backoff (design-integrations.md §1) отсутствует. status.md явно фиксирует Q-001..Q-003 как blocked (auth/protocol/contract — открытые вопросы к IT/ИБ E-Zoo). По правилам ревью — НЕ блокер, только напоминание.

---

## Незначительные замечания

1. **`mappers/erp_to_domain.go` и `mappers/domain_to_response.go` из design-go-layers §1 не созданы.**
   Вместо них есть `mappers/master.go` и `mappers/facts.go` — структурно эквивалентны, но имена не совпадают с design. Не критично.

2. **`pkg/pgxutil/` отсутствует** (design-go-layers §1). Helper-ы `pgx` всё-таки малы и сейчас встроены в `repository/repository.go` (`mapError`, `chooseExec`). OK для MVP.

3. **`constants/` пакет не создан** (design-go-layers §1). Константы вроде `LockTagDailyLoad`, `AdminPathPrefix`, `LimitDefault` разбросаны по соседним пакетам. Косметика.

4. **`migrations/` верхнего уровня нет** — только `internal/features/data_export/sqls/migrations/`. design-go-layers §1 упоминал «или дубликат для CLI», но сейчас CLI-путь покрывается через `docker-compose.yml`-сервис `migrate` с томом на embed-папку — приемлемо.

5. **`exports/service.go` пишет meta-only, без реальной сборки данных.** Это явно прокомментировано (`pending → ready с пустым телом`), и `code-plan-status.md` Phase 14 фиксирует «реальная сборка contents — пост-MVP». OK.

6. **`handler/exports.go:128` — детект `format`-нарушения через `containsAny(err.Error(), "Format","format")`.** Хрупкий способ; `validate.ValidationErrors` даёт типизированный API. Замечание на рефакторинг.

7. **HS256 default `JWTSecret=""`.** `cfg.JWTSecret` имеет `default:""`, не `required:"true"`. Проверка происходит только в `keyFunc` middleware при первом запросе (`return nil, errors.New("JWT secret not configured")`). Безопаснее упасть на старте. design-infrastructure.md помечает `JWT_SECRET` как `required (HS256)` — желательно отразить fail-fast.

8. **Loader не привязан к scheduler advisory lock явно.**
   Lock берётся в `Scheduler.Tick`, и loader.Run отдельно lock не запрашивает (т.е. при ручном `POST /admin/loads` lock держит scheduler через `TriggerOnce` → `Tick`). Корректно по design (single-entry-point), но стоило бы добавить юнит-тест на проверку «вызов loader.Run без lock-а возможен только из Tick».

---

## Соответствие дизайну

| Аспект | Соответствует | Расхождение |
|---|---|---|
| Структура папок (design-go-layers §1) | в основном да | нет `constants/`, `pkg/pgxutil/`; mapper-файлы переименованы (`master.go`/`facts.go` вместо `erp_to_domain.go`/`domain_to_response.go`); 14 entity-handler-ов схлопнуты в `not_implemented.go` (501 заглушка); `data_export/router/router.go` есть |
| Интерфейсы (`SourceReader`, `ExportsStorage`, `Repository`, `LoaderAPI`, `SnapshotRepoAPI`, validator `Engine`) | да | `SourceReader` вернул сигнатуру с `(PageIterator[T], error)` вместо отдельных типизированных Iterator-интерфейсов из design — упрощение через generics, contract эквивалентен; `ReadSupplierStockSnapshot` возвращает `(iterator, error)` без явного `bool present` (Q-009/ADR-010 обрабатывается в reader-stub через empty-iterator + warn) |
| DTO (поля, json-теги) | да | `mappers/master.go` присваивает все поля 1:1; DTO имеют `db:` + `json:` теги; cursor + page DTO покрыты тестами |
| Sentinel errors → mappers | частично | sentinel-набор полный (23 sentinel-а, см. ниже); error-mapping для HTTP лежит inline в хендлерах через `WriteJSON`, отдельного `mappers/errors.go` нет |
| Sentinel coverage (по design-tests.md §6) | да, кроме `ErrInternal` | `ErrBadRequest`, `ErrInvalidCursor`, `ErrInvalidQuery`, `ErrInvalidExportFormat`, `ErrAuthMissingToken/Invalid/Forbidden`, `ErrNotFound`, `ErrLoadNotFound`, `ErrExportNotFound`, `ErrSnapshotNotFound`, `ErrLoadAlreadyRunning`, `ErrCannotRetry`, `ErrSnapshotNotReady`, `ErrERPUnavailable`, `ErrQualityThresholdExceeded`, `ErrAlreadyExists` — покрыты. `ErrInternal` — 0 тестовых файлов |
| go:embed SQL | да | `internal/features/data_export/sqls/queries/embed.go` (32 SQL-файла, паника при отсутствии — defensive); `sqls/migrations/embed.go` для миграций |
| Repository pgx + mapError | да | `mapError`: `pgx.ErrNoRows → ErrNotFound`, `pgconn.PgError 23505 → ErrAlreadyExists` (есть в `repository/repository.go`); используется `pgxpool` (нет `database/sql`) |
| Fiber v3 conventions | да | 35 мест с `c fiber.Ctx` (без указателя), 0 мест `c *fiber.Ctx`; `c.Bind().JSON(&req)` / `c.Bind().Query(&req)` — корректно; ListenConfig используется |
| JWT (HS256/RS256, alg-confusion) | да | `internal/middleware/jwt.go` явно валидирует `t.Method` против `cfg.Alg`, `TestJWT_AlgConfusion_HS256TokenAgainstRS256_Returns401` покрывает это |
| Role middleware (admin-cli / x-flow-etl / it-read) | да | `internal/middleware/role.go` + `role_test.go` |
| Audit middleware (только /admin/*) | да | `audit/writer.go:17 const AdminPathPrefix = "/admin/"`, `if !strings.HasPrefix(path, AdminPathPrefix) { return err }`; тесты `TestAuditMiddleware_PublicPath_DoesNotWrite`, `TestAuditMiddleware_AdminPath_WritesEntry` |
| Atomic snapshot flip (ADR-102) | да | `loader.Run` делает `BeginTx → Flip → MarkCommitted → Commit`, на любом fail — `MarkFailed` с конкретным reason (`flip_begin_tx`, `flip_failed`, `mark_committed_failed`, `commit_failed`) |
| 1% quality threshold (ADR-003) | да | `loader.go:111 if totalLines > 0 && float64(totalFailed)/float64(totalLines) > QualityThresholdRatio` → `MarkFailed(ErrQualityThresholdExceeded.Code)` |
| Advisory lock + 409 | да | `scheduler/scheduler.go` берёт `pg_try_advisory_lock` на отдельном connection и держит до конца Tick; `scheduler_test.go::TestLockKey_FNV_Stable`; параллельный 409 покрыт фазой 08 (по status.md note) |
| No S3 (design-integrations.md §4) | да | `exports_storage/local_fs.go`, `storage.go` — только LocalFS |
| No CI (ADR-012, отложено) | да | `.golangci.yml` есть, GitHub Actions / CI workflow отсутствуют |
| net/http клиент к ERP + retry | НЕТ (отложено) | реализован только in-memory `erp_e_zoo_reader.go` (fixtures); status.md Q-001..Q-003 blocked |
| `log/slog` структурированный лог | да | `internal/logger/logger.go`, `log.InfoContext`, `slog.String/Int64/Any` повсеместно |
| Prometheus `/metrics` | да (зарегистрирован), не инкрементируется (loader/scheduler) | `internal/observability/metrics.go` + `middleware.go` для HTTP; loader-метрики не вызываются |
| `kelseyhightower/envconfig` | да | `internal/config/config.go` |
| `gocron/v2` + cron expression | да | `scheduler/scheduler.go` использует `gocron.CronJob(s.cfg.CronExpr, false)` + `WithSingletonMode(LimitModeReschedule)` |
| Errors через `pkg/errorspkg.WriteJSON` | да | 50+ обращений в `handler/*.go`, `exports/service.go`, и т.д. |
| `fmt.Errorf("...: %w", err)` обёртки | да | повсеместно в loader/scheduler/repository/middleware/app |
| `fmt.Println` / `panic` в production | 1 место | `sqls/queries/embed.go:22 panic(err)` — defensive panic при init `MustRead` (если SQL-файл из embed не найден). По комментарию пакета это интенциональный fail-fast на старте сервиса. Допустимо. `fmt.Println` — отсутствует |
| JWT secret через env | да | `JWT_SECRET` envconfig; в коде секрет не хардкодится |
| ERP креды через env | да | `ERP_API_KEY`, `ERP_OAUTH_*`, `ERP_MTLS_*` — все через envconfig |

---

## Документированные ограничения (НЕ блокеры)

- **Repository select-методы только для `products` + `receipt_line`** — остальные 14 закрыты `NotImplementedHandlers` (501 ожидаемо, но фактически отдают 500 — см. блокер #1). Зафиксировано в `code-plan-status.md` Phase 13.
- **ERP reader = in-memory stub** (`erp_e_zoo_reader.go`). Q-001..Q-003 blocked, ждут IT/ИБ E-Zoo. Phase 09 note.
- **Real export content build = pending → ready с пустым телом.** Phase 14 note.
- **Loader/scheduler integration tests scheduler-а отложены** — параллелизм advisory lock покрыт фазой 08. Phase 12 note.
- **Loader/scheduler метрики зарегистрированы, но инкременты не подставлены** — отметил выше как «Серьёзное замечание», status.md фиксирует это.
- **`master_change_log` diff-логика реализована, но 12 master-сущностей вне products не имеют upsert-методов** → diff пустой для них. Косметика.

---

## Итог

**CHANGES REQUESTED.**

Два блокера:
1. `NotImplementedHandlers` отвечают `500 internal_error` вместо 501 — вводят в заблуждение клиентов и алерты. Лёгкий фикс: добавить `ErrNotImplemented` в `pkg/errorspkg` или использовать `c.SendStatus(fiber.StatusNotImplemented)`.
2. По чек-листу задачи требуется `mappers/` для error-mapping; сейчас mapping инлайн в хендлерах. Если оркестратор согласен, что это design-errors.md уже разрешил inline через `WriteJSON` — блокер можно понизить до серьёзного замечания.

В остальном код хорошо соответствует дизайну: pgx + go:embed + Fiber v3 + atomic flip + 1% threshold + alg-confusion JWT + audit только /admin/* — всё на месте. Sentinel-coverage 22/23 (нет теста только на `ErrInternal`-catch-all).

---

## Ответ оркестратору (5–7 строк)

- **Итог:** CHANGES REQUESTED.
- **Блокеры:** 2 (501→500 баг в `not_implemented.go`; error-mapping в `handler/`, не в `mappers/` — спорный по дизайну).
- **Серьёзные:** 5 (`context.Background()` x5 в проде; `ErrInternal` без теста; loader/scheduler метрики не инкрементируются; 14 select-методов repository = 501; ERP HTTP-клиент = stub).
- **Незначительные:** 8 (имена mapper-файлов, нет `constants/`/`pgxutil/`, нет верхнеуровневой `migrations/`, exports пишет только meta, format-detection через `strings.Contains`, `JWT_SECRET` без `required`, loader-lock неявный, прочая косметика).
- **Готовность к Validation:** не готов до фикса блокера #1 (это однострочник). Блокер #2 — требует уточнения у оркестратора, действительно ли `mappers/` обязателен для error-mapping. Остальное — серьёзные/незначительные, можно править параллельно с Validation.
