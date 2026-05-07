# Phase 08 — Validators (формат запросов)

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Реализовать валидаторы формата для admin-endpoint-ов. Логика — лёгкая (DTO в основном пустые), но точки расширения нужны: `mart_name` для `POST /admin/marts/{name}/refresh`, query-params для `/admin/etl-runs` (status enum), `/admin/reject-log` (severity, entity, cursor пагинация).

## Commit

```
feat(etl_validation): request validators for admin endpoints
```

## Files to CREATE

- `internal/features/etl_validation/validators/validators.go` — `Validator` интерфейс + `New() *Impl`.
- `internal/features/etl_validation/validators/etl_runs.go`:
  - `ValidatePostEtlRun(req *dto.PostEtlRunRequest) error` — body пустой, всегда OK (но сохраняем точку расширения).
  - `ValidateRetryEtlRun(runID string) error` — UUID-формат проверки.
  - `ValidateListEtlRuns(query ListEtlRunsQuery) error` — `status ∈ {running,committed,failed,aborted, ""}`, `kind ∈ {full, mart_refresh, ""}`, `cursor` parse-able timestamp, `limit ∈ [1, 100]`.
  - `ValidateGetEtlRun(id string) error` — UUID-формат.
- `internal/features/etl_validation/validators/marts.go`:
  - `ValidateMartRefresh(name string) error` — единственный допустимый сейчас `mart_supplier_scorecard`. Иначе → `errorspkg.ErrMartRefreshNotSupported` (вернётся EV-005). Список расширяемых mart-ов вычитываем из `constants.RefreshableMarts`.
- `internal/features/etl_validation/validators/reject_log.go`:
  - `ValidateListRejectLog(query ListRejectLogQuery) error` — `severity ∈ {critical, soft, ""}`, `entity ∈ allowedEntities ∪ {""}`, cursor, limit.

### Tests (unit, table-driven)

- `internal/features/etl_validation/validators/validators_test.go` — table-driven для каждого `Validate*`.
  - happy path
  - неверный enum → `errorspkg.ErrBadRequest`
  - mart_refresh с несуществующим именем → `errorspkg.ErrMartRefreshNotSupported`
  - cursor с невалидной датой → `errorspkg.ErrBadRequest`
  - limit вне диапазона → `errorspkg.ErrBadRequest`

## Files to MODIFY

- `internal/features/etl_validation/constants/constants.go` — добавить `RefreshableMarts = []string{MartSupplierScorecard}` и `AllowedEntities` (на основе списка staging-сущностей).

## SQL / Migrations

Нет.

## Run after

```bash
go build ./internal/features/etl_validation/validators/...
go test ./internal/features/etl_validation/validators/... -race -count=1
go vet ./internal/features/etl_validation/validators/...
golangci-lint run ./internal/features/etl_validation/validators/...
```

## Tests

| Test | Что проверяет |
|---|---|
| `TestValidatePostEtlRun` | пустой body OK |
| `TestValidateRetryEtlRun_BadUUID` | invalid UUID → `ErrBadRequest` |
| `TestValidateListEtlRuns_BadStatus` | status='unknown' → `ErrBadRequest` |
| `TestValidateMartRefresh_Supported` | `mart_supplier_scorecard` OK |
| `TestValidateMartRefresh_NotSupported` | `mart_demand_history` → `ErrMartRefreshNotSupported` |
| `TestValidateListRejectLog_BadSeverity` | severity='warning' → `ErrBadRequest` |
| `TestValidateListRejectLog_BadCursor` | invalid timestamp → `ErrBadRequest` |

Coverage пакета `validators` ≥90% (требование integration-test секции из CLAUDE.md распространяется на handlers; для validators — ≥90% unit-тестами).

## Definition of Done

- [ ] Все валидаторы созданы и покрыты table-driven unit-тестами.
- [ ] Каждый сценарий ошибки возвращает корректный sentinel из `errorspkg`.
- [ ] `RefreshableMarts` и `AllowedEntities` зафиксированы в `constants.go`.
- [ ] Coverage ≥90%.
- [ ] `golangci-lint` зелёный.

## Зависимости

Требует фазы 02 (sentinel errors) и 05 (DTO).
