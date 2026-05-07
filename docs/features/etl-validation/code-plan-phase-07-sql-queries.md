# Phase 07 — SQL queries (go:embed)

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Заполнить все SQL-файлы в `internal/features/etl_validation/sqls/` (загружаются через `go:embed`). Включает: queries для `etl_runs`, `reject_log`, `audit_access`, advisory lock, INSERT-FROM-SELECT в mart-таблицы, snapshot flip.

## Commit

```
feat(etl_validation): SQL queries with go:embed (etl_runs, reject_log, marts, snapshot flip)
```

## Files to CREATE / FILL (заполняются финально в этой фазе)

В `internal/features/etl_validation/sqls/`:

### etl_runs
- `etl_runs_insert.sql` — INSERT (id, started_at, status='running', kind, target_mart, source_load_id, parent_run_id, trigger, requester) RETURNING ... .
- `etl_runs_get_by_id.sql` — SELECT всех полей (см. design-sql.md §3.3).
- `etl_runs_list.sql` — SELECT WHERE status/kind/started_at-cursor + LIMIT (см. design-sql.md §3.4).
- `etl_runs_update_status.sql` — UPDATE etl_runs SET status, finished_at, committed_at, marts_summary, failure_reason, lines_total, lines_failed WHERE id.
- `etl_runs_get_current_running.sql` — SELECT … FROM marts.etl_runs WHERE status='running' ORDER BY started_at DESC LIMIT 1.

### reject_log
- `reject_log_bulk_insert.sql` — INSERT … VALUES (multi-row), либо использовать `pgx.CopyFrom` без отдельного SQL.
- `reject_log_list.sql` — SELECT … WHERE etl_run_id, entity, severity, created_at-cursor; LIMIT.

### audit_access
- `audit_access_insert.sql` — INSERT (method, path, sub, role, request_id, status_code, latency_ms, accessed_at).

### advisory lock
- `advisory_lock_try.sql` — `SELECT pg_try_advisory_xact_lock($1::bigint)` (key — hash of `'etl-run'`).

### staging
- `staging_create_temp_tables.sql` — CREATE TEMP TABLE pg_temp.stg_* (по сущностям источника: products, locations, suppliers, stock_on_hand, order_rule, supply_spec, demand_events).

### marts (INSERT-FROM-SELECT)
- `mart_demand_history_append.sql` — INSERT INTO marts.mart_demand_history (...) SELECT ..., $1::uuid AS etl_run_id, $2::uuid AS source_load_id FROM pg_temp.stg_demand_events WHERE event_time >= $3 AND event_time < $4.
- `mart_calculation_input_truncate_insert.sql` — TRUNCATE marts.mart_calculation_input; INSERT WITH stock/rule_priority/chosen CTE (см. design-sql.md §3.8).
- `mart_kpi_daily_append.sql`.
- `mart_master_current_truncate_insert.sql`.
- `mart_supplier_scorecard_truncate_insert.sql`.

### snapshot flip
- `snapshot_flip.sql` (если применимо для атомарного перехода): UPDATE marts.etl_runs SET status='committed', committed_at=NOW(), finished_at=NOW(), lines_total=$2, lines_failed=$3, marts_summary=$4 WHERE id=$1.
- `cleanup_failed_runs.sql` — UPDATE marts.etl_runs SET status='aborted', failure_reason='process_killed' WHERE status='running' AND started_at < NOW() - INTERVAL '6 hours' (опционально для recovery).

## Files to MODIFY

- (нет — все SQL в этой фазе создаются с финальным содержимым)

## SQL / Migrations

См. полный текст всех queries в `design-sql.md` §3 и §3.8.

## Run after

```bash
go build ./internal/features/etl_validation/...
go test -tags=integration ./internal/features/etl_validation/repository/... -race -count=1
psql $ETL_DSN -f internal/features/etl_validation/sqls/etl_runs_insert.sql --dry-run  # smoke
```

## Tests

- Все integration-тесты репозитория (фаза 06) теперь работают с финальным SQL и должны зелёные.
- Дополнительный тест в `repository/marts_integration_test.go`: после applied marts query — `SELECT count(*) FROM marts.mart_demand_history WHERE etl_run_id=$1` совпадает с ожидаемым.

## Definition of Done

- [ ] Все .sql файлы созданы и содержат корректные queries (без placeholder-ов вида `SELECT 1`).
- [ ] `embed.FS` корректно подхватывает все файлы.
- [ ] Все integration-тесты репозитория проходят с финальным SQL.
- [ ] CTE `rule_priority` в `mart_calculation_input_truncate_insert.sql` корректно резолвит `applicable_rule_kind` (`order_rule > supply_spec` per ADR-024).
- [ ] `golangci-lint`/`go vet` зелёные.

## Зависимости

Требует фазу 06 (репозиторий уже использует placeholder-ы).
