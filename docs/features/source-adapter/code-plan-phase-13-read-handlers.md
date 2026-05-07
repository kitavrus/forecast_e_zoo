# Phase 13: HTTP read-handlers /v1/{entity} + /v1/snapshots/current + healthz + NDJSON streaming + ETag

**Цель:** реализовать все 16+1 read-handler-ов: 12 master-сущностей + 4 факта + `master_change_log` + `store_assortment_lifecycle_events` + `snapshots/current` + `healthz`. Все они: подписаны JWT (роль `x-flow-etl` или `it-read`), пагинированы курсором, поддерживают `application/x-ndjson` стриминг, добавляют `X-Snapshot-Id`, `X-Load-Id`, `ETag`, `Cache-Control: private, max-age=86400`.

**Commit:** `feat(data_export/handler): read-handlers /v1/* + NDJSON streaming + ETag + healthz`

---

## Files to CREATE

### Базовые хелперы

- `internal/features/data_export/handler/streaming.go`:
  - `func StreamNDJSON[T any](c fiber.Ctx, items []T) error` — выставляет `Content-Type: application/x-ndjson` и пишет построчно.
  - `func WritePageHeaders(c fiber.Ctx, snapshotID uuid.UUID, loadID uuid.UUID, etag string)`.
- `internal/features/data_export/handler/etag.go`:
  - `func ComputeETag(loadID uuid.UUID, entity string, lastModified time.Time) string` — `W/"<sha256>"`.
  - Проверка `If-None-Match` → 304.

### Handlers (по одному файлу)

- `internal/features/data_export/handler/healthz.go` — `GET /healthz`. Проверяет `pool.Ping(ctx)`. 200 `{status:"ok",db:"ok"}` или 503.
- `internal/features/data_export/handler/snapshots.go` — `GET /v1/snapshots/current`. 200 + `GetSnapshotsCurrentResponse`. 503 `ErrSnapshotNotReady`.
- `internal/features/data_export/handler/products.go` — `GET /v1/products?cursor=&limit=`.
- `internal/features/data_export/handler/product_barcodes.go`
- `internal/features/data_export/handler/category.go`
- `internal/features/data_export/handler/location.go`
- `internal/features/data_export/handler/supplier.go`
- `internal/features/data_export/handler/store_assortment.go`
- `internal/features/data_export/handler/store_assortment_lifecycle.go` — `GET /v1/store_assortment/lifecycle_events`.
- `internal/features/data_export/handler/master_change_log.go` — `GET /v1/master_change_log?entity=&since=`.
- `internal/features/data_export/handler/supply_spec.go`
- `internal/features/data_export/handler/promo.go`
- `internal/features/data_export/handler/order_rule.go`
- `internal/features/data_export/handler/supply_plan.go`
- `internal/features/data_export/handler/receipt_line.go` — обязательный `event_date_from` / `event_date_to`.
- `internal/features/data_export/handler/location_stock_snapshot.go`
- `internal/features/data_export/handler/stock_movement.go`
- `internal/features/data_export/handler/supplier_stock_snapshot.go`

Каждый handler:
1. JWT + role-check (`x-flow-etl` или `it-read`) — вешается в роутере (фаза 15).
2. Парсит cursor / limit / фильтры через `validators` (фаза 07).
3. Берёт `currentLoadID` из `snapshot.Service.Current` (если NotReady → 503).
4. Зовёт `repository.SelectXxx(ctx, currentLoadID, cursor, limit, ...)`.
5. Считает ETag, проверяет `If-None-Match` → 304.
6. Stream NDJSON.

### Тесты

- `handler/healthz_test.go` — unit:
  - `TestHealthz_OK_DBPing`
  - `TestHealthz_DBError_503`
- `handler/snapshots_test.go` — unit:
  - `TestSnapshotsCurrent_HappyPath`
  - `TestSnapshotsCurrent_NotReady_503`
- `handler/products_test.go` — unit (и mock-repository):
  - `TestProducts_HappyPath_Returns200_NDJSON`
  - `TestProducts_BadCursor_Returns400`
  - `TestProducts_IfNoneMatchSameETag_Returns304`
  - `TestProducts_SnapshotNotReady_503`
- `handler/receipt_line_test.go`:
  - `TestReceiptLine_RequiresEventDateRange_Returns400IfMissing`
  - `TestReceiptLine_DateRange_HappyPath`
- `handler/etag_test.go` — `TestComputeETag_DeterministicForSameInputs`, `TestComputeETag_ChangesWhenLoadIDChanges`.
- `handler/streaming_test.go` — `TestStreamNDJSON_WritesLineByLine`.

Остальные 12 handler-ов покрываем shared-tests (`generic_handler_test.go`) — параметризованный тест проверяет happy path + bad cursor + snapshot not ready (т.е. 36 кейсов на 12 сущностей).

## Files to MODIFY

- `internal/features/data_export/handler/mappers/error_mapper.go` — добавить ETag/304 как special case.

## SQL/Migrations

— нет.

## Run after

```bash
make build
make test-unit
make lint
```

## Tests in this phase

- 2 теста healthz
- 2 теста snapshots
- 4 теста products (специфичные)
- 2 теста receipt_line (специфичные)
- 2 теста etag
- 1 тест streaming
- 36 параметризованных кейсов для остальных 12 master/facts handler-ов

Итого: ~50.

## Definition of Done

- [ ] Все 18 endpoints реализованы.
- [ ] NDJSON streaming работает (Content-Type, line-by-line).
- [ ] ETag/If-None-Match → 304 корректно.
- [ ] Snapshot not ready → 503 + Retry-After.
- [ ] receipt_line / facts требуют event_date range.
- [ ] healthz пингует БД.
- [ ] `make build` / `make test-unit` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/handler): read handlers ...`.
