# Phase 09: SourceReader interface + erp_e_zoo_reader stub

**Цель:** ввести интерфейс `SourceReader` (порт к ERP) и **in-memory стаб** `erp_e_zoo_reader` для MVP. Реальная ERP-интеграция блокируется Q-001/Q-002/Q-003 (выбор контракта/auth) — стаб разблокирует разработку всех нижестоящих слоёв и тестов. Структура такая, чтобы заменить стаб на REST/SOAP/SFTP реализацию без изменения loader-а.

**Commit:** `feat(data_export/loader): SourceReader interface + erp_e_zoo_reader stub (in-memory MVP)`

---

## Files to CREATE

### Интерфейс

- `internal/features/data_export/loader/source_reader.go`:
  - `type SourceReader interface { ReadProducts(ctx, since time.Time) (PageIterator[Product], error); ReadProductBarcodes(...); ... ReadReceiptLine(ctx, dateFrom, dateTo time.Time) (PageIterator[ReceiptLine], error); ReadLocationStockSnapshot(...); ReadStockMovement(...); ReadSupplierStockSnapshot(...) /* optional */; Close(ctx) error }` — по методу на каждую из 16 сущностей.
  - `type PageIterator[T any] interface { Next(ctx) bool; Item() T; Err() error; Close() error }`.
  - `type SourceAuth interface { Apply(req *http.Request) error }` (заготовка для будущих REST/SOAP реализаций; для in-memory стаба не нужна, но интерфейс публикуем).

### In-memory стаб

- `internal/features/data_export/loader/erp_e_zoo_reader.go`:
  - `type ErpEZooReader struct { fixtures Fixtures }` — backend на slice-ах из загруженных JSON-fixtures.
  - `New(fixturesPath string) (*ErpEZooReader, error)` — читает `testdata/fixtures/*.json` (или env-конфиг путь).
  - Реализует все методы `SourceReader` — каждый возвращает `sliceIterator[T]` поверх соответствующего slice.
  - Поддерживает `since` (фильтр по `updated_at`) и `event_date` ranges.
  - Опционально `supplier_stock_snapshot` возвращает пустой набор (см. ADR-010, флаг `entity_optional`).
- `internal/features/data_export/loader/iterator.go`:
  - `type sliceIterator[T any] struct{ items []T; idx int; err error }` + методы `Next`, `Item`, `Err`, `Close`.

### Fixtures (для стаба и тестов)

- `testdata/fixtures/products.json` — 5–10 строк с реалистичными данными.
- `testdata/fixtures/product_barcodes.json`
- `testdata/fixtures/category.json`
- `testdata/fixtures/location.json` (3 магазина)
- `testdata/fixtures/supplier.json` (2 поставщика)
- `testdata/fixtures/supply_spec.json`
- `testdata/fixtures/promo.json`
- `testdata/fixtures/order_rule.json`
- `testdata/fixtures/supply_plan.json`
- `testdata/fixtures/store_assortment.json`
- `testdata/fixtures/store_assortment_lifecycle_events.json`
- `testdata/fixtures/master_change_log.json`
- `testdata/fixtures/receipt_line.json` (~50 строк, разные `event_date`)
- `testdata/fixtures/location_stock_snapshot.json`
- `testdata/fixtures/stock_movement.json`
- `testdata/fixtures/supplier_stock_snapshot.json` (пустой `[]` — проверка optional поведения).

### Тесты (unit, без БД)

- `internal/features/data_export/loader/erp_e_zoo_reader_test.go`:
  - `TestErpEZooReader_ReadProducts_HappyPath`
  - `TestErpEZooReader_ReadProducts_SinceFilter`
  - `TestErpEZooReader_ReadReceiptLine_DateRangeFilter`
  - `TestErpEZooReader_SupplierStock_EmptyOK` — optional сущность не падает.
  - `TestPageIterator_NextThenItem`
  - `TestPageIterator_AfterCloseReturnsFalse`

## Files to MODIFY

- `internal/features/data_export/loader/source_reader.go` — добавить package-level комментарий с пояснением, что ERP-stack/auth не выбран (см. Q-001..Q-003) и что REST/SOAP/SFTP-реализация — отдельная фаза вне MVP.

## SQL/Migrations

— нет.

## Run after

```bash
make build
make test-unit
make lint
```

## Tests in this phase

- 6 unit-тестов в `erp_e_zoo_reader_test.go` (см. список выше).

## Definition of Done

- [ ] Интерфейс `SourceReader` покрывает все 16 сущностей.
- [ ] `PageIterator[T]` — generic, без аллокаций на каждом `Next`.
- [ ] `ErpEZooReader` загружает все 16 fixtures без ошибок.
- [ ] `supplier_stock_snapshot` корректно отдаёт пустой итератор (optional).
- [ ] `since` и date-range фильтры работают.
- [ ] `make build` / `make test-unit` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/loader): SourceReader ...`.
