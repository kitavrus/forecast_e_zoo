# Phase 10 — Extractor (HTTP клиент к source-adapter)

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Реализовать HTTP-клиент к API source-adapter (`net/http` без сторонних библиотек, ADR-100):
- JWT bearer (`x-flow-etl` role), HS256 либо RS256.
- ETag / `If-None-Match` для re-fetch оптимизации.
- Retry с экспоненциальным backoff (cap 30s через `ETL_RETRY_BACKOFF_CAP`).
- NDJSON streaming с buffered scanner (1 MiB).
- Mock interfaces для unit-тестов pipeline-а.

## Commit

```
feat(etl_validation): HTTP extractor (snapshots + entities NDJSON streaming + JWT + retry)
```

## Files to CREATE

### Production code

- `internal/features/etl_validation/extractor/client.go`:
  - `Client` struct — `http.Client`, `baseURL`, `tokenSource TokenSource`, `retryCap time.Duration`, `logger *slog.Logger`.
  - `Do(ctx, req *http.Request) (*http.Response, error)` — добавляет Authorization: Bearer, делает retry с backoff, маппит ошибки в `errorspkg.ErrSourceUnavailable` после исчерпания retry.
  - `NewClient(cfg ClientConfig, ts TokenSource, l *slog.Logger) *Client`.
- `internal/features/etl_validation/extractor/snapshots.go`:
  - `SnapshotsClient` интерфейс: `GetCurrent(ctx) (Snapshot, error)`. Возвращает `errorspkg.ErrSnapshotNotReady` при HTTP 503.
  - Реализация `snapshotsClient` оборачивает `Client`.
  - `Snapshot` тип: `LoadID string`, `CreatedAt time.Time`.
- `internal/features/etl_validation/extractor/entities.go`:
  - `EntitiesClient` интерфейс: `Stream(ctx, entity, snapshotID, etag string) (NDJSONReader, error)`.
  - `NDJSONReader` интерфейс — `Next(target any) error` (io.EOF on end), `ETag() string`, `Close() error`.
  - Реализация на `bufio.Scanner` с increased buffer 1 MiB.
- `internal/features/etl_validation/extractor/token_source.go`:
  - `TokenSource` интерфейс — `Token(ctx) (string, error)`.
  - `HS256TokenSource` — генерирует JWT с claims `{role: "x-flow-etl", iss: "etl", exp}` через `golang-jwt/jwt/v5` с `ETL_JWT_SIGNING_KEY`.
  - `RS256TokenSource` — то же, но с RSA private key из `ETL_JWT_PRIVATE_KEY_PATH`.
  - Token caching (TTL 5 мин < exp).
- `internal/features/etl_validation/extractor/mocks.go`:
  - `MockSnapshotsClient`, `MockEntitiesClient` — для unit-тестов pipeline-а (используют canned responses).

### Tests (unit)

- `internal/features/etl_validation/extractor/client_test.go`:
  - `TestClient_Do_RetryOn5xx` — `httptest.Server`, первые 2 ответа 503, третий 200 → success.
  - `TestClient_Do_RetryExhausted` — все ответы 503 → `ErrSourceUnavailable`.
  - `TestClient_Do_BackoffCap` — фактический backoff не превышает 30s.
  - `TestClient_Do_AuthHeaderSet` — `Authorization: Bearer <jwt>` присутствует.
- `internal/features/etl_validation/extractor/snapshots_test.go`:
  - `TestSnapshots_GetCurrent_OK` — JSON parsing.
  - `TestSnapshots_GetCurrent_503` → `ErrSnapshotNotReady`.
- `internal/features/etl_validation/extractor/entities_test.go`:
  - `TestEntities_Stream_NDJSON` — поток из `httptest.Server`, 100 строк, корректный unmarshal.
  - `TestEntities_Stream_BigLine_1MiB` — строка >1 MiB обрабатывается без panic (либо ошибка с понятным сообщением).
  - `TestEntities_Stream_ETag` — ETag прокидывается в response.
- `internal/features/etl_validation/extractor/token_source_test.go`:
  - `TestHS256TokenSource_Token` — JWT валиден (parse-able), claims корректны, exp в будущем.
  - `TestHS256TokenSource_Caching` — повторный вызов в TTL не пересоздаёт токен.

## Files to MODIFY

- `go.mod` — `github.com/golang-jwt/jwt/v5` (если уже не подключён в фазе 01).

## SQL / Migrations

Нет.

## Run after

```bash
go build ./internal/features/etl_validation/extractor/...
go test ./internal/features/etl_validation/extractor/... -race -count=1
golangci-lint run ./internal/features/etl_validation/extractor/...
```

## Tests

Все unit-тесты — через `httptest.Server`. Integration test против реального source-adapter — отложен (E2E на Validation-стадии).

## Definition of Done

- [ ] `Client`, `SnapshotsClient`, `EntitiesClient`, `TokenSource` реализованы.
- [ ] Retry с backoff cap 30s работает (тестируемое exponential).
- [ ] NDJSON streaming с 1 MiB buffer работает.
- [ ] JWT генерация HS256 + RS256 покрыта тестами.
- [ ] Mock interfaces реализованы и используются в тестах последующих фаз.
- [ ] Coverage ≥85%.
- [ ] `golangci-lint`/`go vet` зелёные.

## Зависимости

Требует фаз 01 (config), 02 (sentinel errors).
