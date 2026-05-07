# Phase 09 — Validation engine reuse + etl_validation_rules.yaml

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Подключить YAML-driven validation engine из Модуля 1 (`internal/features/data_export/validation`) как library и реализовать обёртку (engine_adapter) для cross-entity rules: `fk_exists`, `unique_business_key`, `aggregate_sum_matches`, `referential_integrity`, `null_required_field`. Заполнить `configs/etl_validation_rules.yaml`.

## Commit

```
feat(etl_validation): YAML-driven validation engine adapter + cross-entity rules
```

## Files to CREATE

- `internal/features/etl_validation/validation/engine_adapter.go`:
  - `Engine` интерфейс — `Run(ctx, dataset Dataset) (Report, error)`.
  - `Adapter` обёртка — оборачивает `dataexportvalidation.Engine`, регистрирует новые builtin checks, передаёт YAML.
  - `Dataset` тип — staging-таблицы, доступные через pgx tx.
  - `Report` тип — `LinesTotal`, `LinesFailed`, `Entries []RejectLogEntry`.
- `internal/features/etl_validation/validation/builtin_fk_exists.go`:
  - `FkExistsRule` — для каждой строки entity A проверяет, что значение FK-колонки присутствует в entity B (staging-таблица).
- `internal/features/etl_validation/validation/builtin_unique_bkey.go`:
  - `UniqueBusinessKeyRule` — обнаруживает дубликаты по составному business-ключу.
- `internal/features/etl_validation/validation/builtin_aggregate_sum.go`:
  - `AggregateSumMatchesRule` — `SUM(column1) WHERE filter == SUM(column2) WHERE filter` (для cross-entity invariant).
- `internal/features/etl_validation/validation/builtin_referential_integrity.go`:
  - `ReferentialIntegrityRule` — поверхностная проверка `(parent_table, parent_pk_column) ↔ (child_table, child_fk_column)` через staging.
- `internal/features/etl_validation/validation/builtin_null_required.go`:
  - `NullRequiredFieldRule` — required-поля не NULL.
- `internal/features/etl_validation/validation/severity.go` — `Severity` enum (`critical`, `soft`).
- `internal/features/etl_validation/validation/registry.go` — DSL: `RegisterRule(name, factory)` + список встроенных правил, мапинг `kind` → factory.

### Tests (unit)

- `internal/features/etl_validation/validation/builtin_fk_exists_test.go` — table-driven с in-memory dataset.
- `internal/features/etl_validation/validation/builtin_unique_bkey_test.go`.
- `internal/features/etl_validation/validation/builtin_aggregate_sum_test.go`.
- `internal/features/etl_validation/validation/builtin_referential_integrity_test.go`.
- `internal/features/etl_validation/validation/builtin_null_required_test.go`.
- `internal/features/etl_validation/validation/engine_adapter_test.go` — golden YAML → expected reject entries (фикстура `testdata/rules.yaml`, `testdata/dataset.json`, `testdata/expected.json`).

### Tests (integration)

- `internal/features/etl_validation/validation/engine_integration_test.go` — реальные данные в staging-таблицах через dockertest, прогон engine, ожидаемые reject-entries в `marts.reject_log`.

## Files to MODIFY

- `configs/etl_validation_rules.yaml` — заполнить полным набором правил для MVP:
  ```yaml
  version: 1
  rules:
    - name: products_unique_id
      kind: unique_business_key
      entity: products
      severity: critical
      keys: [id]
    - name: stock_fk_product
      kind: fk_exists
      entity: stock_on_hand
      severity: critical
      column: product_id
      ref_entity: products
      ref_column: id
    - name: stock_fk_location
      kind: fk_exists
      entity: stock_on_hand
      severity: critical
      column: location_id
      ref_entity: locations
      ref_column: id
    - name: order_rule_fk_supplier
      kind: fk_exists
      entity: order_rule
      severity: soft
      column: supplier_id
      ref_entity: suppliers
      ref_column: id
    - name: demand_required_qty
      kind: null_required_field
      entity: demand_events
      severity: critical
      column: qty
    # ...прочие правила
  ```
- `internal/features/data_export/validation/...` — если требуется экспорт публичного API (типы `Rule`, `Engine`), добавить (но без модификаций бизнес-логики). Если уже public — modify не требуется.

## SQL / Migrations

Нет.

## Run after

```bash
go build ./internal/features/etl_validation/validation/...
go test ./internal/features/etl_validation/validation/... -race -count=1
go test -tags=integration ./internal/features/etl_validation/validation/... -race -count=1
golangci-lint run ./internal/features/etl_validation/validation/...
```

## Tests

| Test | Что проверяет |
|---|---|
| Все builtin_*_test | table-driven happy/error |
| `TestEngine_GoldenFile` | golden YAML + dataset → expected report |
| `TestEngine_Integration_Postgres` | реальные staging-таблицы → reject_log |
| `TestEngine_QualityThresholdMath` | `LinesFailed/LinesTotal` подсчитан корректно |

## Definition of Done

- [ ] Engine adapter подключает 5 builtin rules через registry.
- [ ] `configs/etl_validation_rules.yaml` валидный (parse без ошибок при старте).
- [ ] Все unit-тесты builtin rules проходят.
- [ ] Integration test с реальным Postgres проходит.
- [ ] Coverage пакета validation ≥85%.
- [ ] Engine не зависит от cyclic imports (`internal/features/etl_validation/validation` → `internal/features/data_export/validation` — однонаправленно).
- [ ] `golangci-lint` зелёный.

## Зависимости

Требует фазы 05 (models), 06 (repository, для integration test через staging-таблицы), 07 (SQL). Использует existing engine из Модуля 1.
