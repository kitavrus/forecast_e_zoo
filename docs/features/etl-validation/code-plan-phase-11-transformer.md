# Phase 11 — Transformer (5 mart builders)

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Реализовать 5 mart-builder-ов: `DemandHistoryBuilder`, `CalculationInputBuilder`, `KpiDailyBuilder`, `MasterCurrentBuilder`, `SupplierScorecardBuilder`. Каждый — SQL-driven (выполняет `INSERT … SELECT FROM staging` через `repository.Marts.Upsert*`). Логика `applicable_rule_id` (priority `order_rule > supply_spec`, ADR-024) живёт здесь.

## Commit

```
feat(etl_validation): 5 mart builders (transformer) with applicable_rule_id resolver
```

## Files to CREATE

### Production code

- `internal/features/etl_validation/transformer/builder.go`:
  - `Builder` интерфейс — `Build(ctx, tx pgx.Tx, runID, sourceLoadID string) (RowsWritten int64, err error)`.
  - `Name() string` — имя mart-а.
- `internal/features/etl_validation/transformer/demand_history.go`:
  - `DemandHistoryBuilder` — append-семантика, INSERT FROM SELECT (filter по period: текущий + предыдущий месяц для bi-temporal MVP append-only).
- `internal/features/etl_validation/transformer/calculation_input.go`:
  - `CalculationInputBuilder` — TRUNCATE + INSERT с CTE `rule_priority` (order_rule > supply_spec, ADR-024).
- `internal/features/etl_validation/transformer/kpi_daily.go`:
  - `KpiDailyBuilder` — append-семантика по дням за prev N days.
- `internal/features/etl_validation/transformer/master_current.go`:
  - `MasterCurrentBuilder` — TRUNCATE + INSERT (snapshot текущих справочников).
- `internal/features/etl_validation/transformer/supplier_scorecard.go`:
  - `SupplierScorecardBuilder` — TRUNCATE + INSERT (запускается только из `mart_refresh`, не в полном pipeline). Маркируется флагом `OnDemandOnly = true`.
- `internal/features/etl_validation/transformer/registry.go`:
  - `Registry` — мапа `name → Builder`. `BuildersForFullRun()` возвращает все кроме OnDemandOnly. `BuilderByName(name)` для admin-refresh.

### Tests (unit + integration)

- `internal/features/etl_validation/transformer/calculation_input_test.go` — golden-data integration:
  - dockertest postgres + applied миграции 1001/1002.
  - Заполняем staging-таблицы fixture-данными.
  - Прогоняем `CalculationInputBuilder.Build(ctx, tx, runID, sourceLoadID)`.
  - Проверяем `mart_calculation_input.applicable_rule_kind` — для пары с `order_rule` → `'order_rule'`, без `order_rule` но с `supply_spec` → `'supply_spec'`, без обоих → `'none'`.
- `internal/features/etl_validation/transformer/demand_history_test.go` — append-семантика, два прогона с разными `runID`/`sourceLoadID` → по 2 записи на (product_id, location_id, event_time).
- `internal/features/etl_validation/transformer/kpi_daily_test.go`.
- `internal/features/etl_validation/transformer/master_current_test.go` — TRUNCATE + INSERT, после повторного прогона прежние строки исчезли.
- `internal/features/etl_validation/transformer/supplier_scorecard_test.go`.
- `internal/features/etl_validation/transformer/registry_test.go` — `BuildersForFullRun()` не содержит `mart_supplier_scorecard`.

## Files to MODIFY

- (нет — все добавления в новых файлах)

## SQL / Migrations

Builder-ы используют SQL из фазы 07 (`mart_*_truncate_insert.sql`, `mart_*_append.sql`). Нового SQL не добавляется.

## Run after

```bash
go build ./internal/features/etl_validation/transformer/...
go test -tags=integration ./internal/features/etl_validation/transformer/... -race -count=1
golangci-lint run ./internal/features/etl_validation/transformer/...
```

## Tests

| Test | Что проверяет |
|---|---|
| `TestCalculationInput_OrderRulePriority` | ADR-024: order_rule > supply_spec |
| `TestCalculationInput_SupplySpecFallback` | only supply_spec → applicable_rule_kind='supply_spec' |
| `TestCalculationInput_NoRule` | nothing → applicable_rule_kind='none' |
| `TestDemandHistory_Append` | два runID — две записи |
| `TestKpiDaily_Append` | per-day rows |
| `TestMasterCurrent_Truncate` | повторный прогон — старые строки исчезают |
| `TestSupplierScorecard_Truncate` | (OnDemand) |
| `TestRegistry_BuildersForFullRun` | scorecard НЕ возвращается |

## Definition of Done

- [ ] Все 5 builder-ов реализованы и прошли integration-тесты.
- [ ] `applicable_rule_id` resolver покрыт тремя сценариями (order_rule, supply_spec, none).
- [ ] Registry корректно разделяет full-run/on-demand mart-ы.
- [ ] Coverage ≥85%.
- [ ] `golangci-lint`/`go vet` зелёные.

## Зависимости

Требует фаз 06 (repository.Marts), 07 (SQL queries).
