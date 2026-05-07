# Phase 02 — Sentinel errors EV-*

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Расширить `pkg/errorspkg` пятью новыми sentinel-ошибками с `EV-*` supportMessage кодами для нужд Модуля 2. Reuse существующих (`ErrSnapshotNotReady`, `ErrQualityThresholdExceeded`, `ErrUnauthorized`, `ErrForbidden`, `ErrBadRequest`, `ErrInternal`) — без изменений.

## Commit

```
feat(errorspkg): add EV-001..EV-005 sentinels for etl_validation module
```

## Files to CREATE

- (нет — все правки в существующем `pkg/errorspkg`)

## Files to MODIFY

- `pkg/errorspkg/errors.go` — добавить sentinel-объявления (см. design-errors.md §4):
  ```go
  var (
      // ETL/Marts module (EV-*)
      ErrEtlRunAlreadyRunning    = newSentinel("etl_run_already_running",    http.StatusConflict,    "EV-001")
      ErrEtlRunNotFound          = newSentinel("etl_run_not_found",          http.StatusNotFound,    "EV-002")
      ErrCannotRetryEtl          = newSentinel("cannot_retry_etl",           http.StatusConflict,    "EV-003")
      ErrSourceUnavailable       = newSentinel("source_unavailable",         http.StatusBadGateway,  "EV-004")
      ErrMartRefreshNotSupported = newSentinel("mart_refresh_not_supported", http.StatusBadRequest,  "EV-005")
  )
  ```
- `pkg/errorspkg/codes.go` (`SupportMessageCodes`) — добавить `"EV-001"`, `"EV-002"`, `"EV-003"`, `"EV-004"`, `"EV-005"` в общий список (см. CLAUDE.md §8.1).
- `pkg/errorspkg/error_response.go` — расширить `enums:` тег и `// @Description` блок поля `SupportMessage` (добавить пять новых кодов с описаниями).
- `pkg/errorspkg/codes_test.go` — sync-тест: значения `enums:"..."` тега у `ErrorResponse.SupportMessage` ⇔ `SupportMessageCodes` (через reflect).
- `pkg/errorspkg/errors_test.go` — добавить unit-тесты на новые sentinel-ы: `errors.Is`, корректный `HTTPStatus()`, `Code()`, `SupportMessage()`.

## SQL / Migrations

Нет.

## Run after

```bash
go build ./...
go vet ./pkg/errorspkg/...
go test ./pkg/errorspkg/... -race -count=1
golangci-lint run ./pkg/errorspkg/...
```

## Tests

| Sentinel | Test |
|---|---|
| `ErrEtlRunAlreadyRunning` | code=`etl_run_already_running`, http=409, support=`EV-001` |
| `ErrEtlRunNotFound`        | code=`etl_run_not_found`, http=404, support=`EV-002` |
| `ErrCannotRetryEtl`        | code=`cannot_retry_etl`, http=409, support=`EV-003` |
| `ErrSourceUnavailable`     | code=`source_unavailable`, http=502, support=`EV-004` |
| `ErrMartRefreshNotSupported` | code=`mart_refresh_not_supported`, http=400, support=`EV-005` |

Sync-тест `codes_test.go` падает, если в `enums:` забыт код или есть лишний.

## Definition of Done

- [ ] Пять sentinel-ов добавлены в `pkg/errorspkg/errors.go`.
- [ ] `SupportMessageCodes` содержит EV-001..EV-005.
- [ ] `enums:` тег и swag `@Description` обновлены в `error_response.go`.
- [ ] `codes_test.go` sync-тест проходит.
- [ ] `errors_test.go` для новых sentinel-ов проходит.
- [ ] `go test ./pkg/errorspkg/... -race -count=1` зелёный.
- [ ] `golangci-lint` без нарушений.
- [ ] CLAUDE.md §8 (таблица supportMessage) обновлять не нужно — это feature-локальные коды; обновление таблицы отнесено в фазу 16 (вместе с финальным observability docs).

## Зависимости

Не требует БД, инфры, фич Модуля 2 — самодостаточная фаза.
