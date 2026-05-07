# Phase 06: SQL queries (go:embed)

**Цель:** создать все SQL-запросы фичи как `.sql`-файлы с go:embed. Покрыть: select-ы master/facts с курсорной пагинацией, insert/update в `loads`, advisory lock, atomic snapshot flip, insert в `reject_log`, чтения `audit_access`, выборки lifecycle/change-log. Repository (фаза 08) их сложит в pgx.

**Commit:** `feat(data_export/sqls): SQL-queries (selects, advisory lock, snapshot flip, reject_log)`

---

## Files to CREATE

### Embed loader

- `internal/features/data_export/sqls/queries/embed.go` — `package queries`, `//go:embed *.sql` `var FS embed.FS`, хелпер `Get(name string) string`.

### Master selects (16 файлов, по одному на сущность с пагинацией по `(load_id, pk)`)

- `queries/select_products.sql`
- `queries/select_product_barcodes.sql`
- `queries/select_category.sql`
- `queries/select_location.sql`
- `queries/select_supplier.sql`
- `queries/select_supply_spec.sql`
- `queries/select_promo.sql`
- `queries/select_order_rule.sql`
- `queries/select_supply_plan.sql`
- `queries/select_store_assortment.sql`
- `queries/select_store_assortment_lifecycle_events.sql`
- `queries/select_master_change_log.sql`

Каждый файл:
```sql
-- $1 = current_load_id (UUID), $2 = after_pk (text), $3 = limit (int)
SELECT ... FROM <entity>
 WHERE load_id = $1 AND pk_text > $2
 ORDER BY pk_text
 LIMIT $3;
```

### Facts selects (4 файла, обязательно фильтр по `event_date`)

- `queries/select_receipt_line.sql` (фильтр `event_date BETWEEN $4 AND $5` + cursor `(event_date, id)`).
- `queries/select_location_stock_snapshot.sql`
- `queries/select_stock_movement.sql`
- `queries/select_supplier_stock_snapshot.sql`

### Loads / snapshot / reject

- `queries/loads_insert_running.sql` — `INSERT INTO loads (load_id, started_at, status, source) VALUES ($1, now(), 'running', $2) RETURNING ...`.
- `queries/loads_mark_committed.sql` — `UPDATE loads SET status='committed', committed_at=now(), lines_total=$2, lines_failed=$3, entity_stats=$4 WHERE load_id=$1 AND status='running'`.
- `queries/loads_mark_failed.sql` — `UPDATE loads SET status='failed', failed_at=now(), failed_reason=$2 WHERE load_id=$1 AND status='running'`.
- `queries/loads_mark_aborted.sql` — `UPDATE loads SET status='aborted', failed_at=now(), failed_reason='stale' WHERE status='running' AND started_at < now() - $1::interval`.
- `queries/loads_get_by_id.sql`
- `queries/loads_select_running.sql` — `SELECT load_id FROM loads WHERE status='running' LIMIT 1`.
- `queries/snapshot_select_current.sql`
- `queries/snapshot_flip.sql` — `UPDATE snapshot_pointer SET previous_load_id=current_load_id, current_load_id=$1, committed_at=now() WHERE id=1 RETURNING ...`. Вызывается **внутри транзакции** loader-а (фаза 10).
- `queries/snapshot_seed.sql` — `INSERT INTO snapshot_pointer(id) VALUES (1) ON CONFLICT DO NOTHING`. Вызывается одноразово на старте сервиса.
- `queries/advisory_lock_try.sql` — `SELECT pg_try_advisory_lock($1::bigint)` (ключ `daily-load` хешируем в bigint). Хеш-функцию (FNV-64) держим в Go-коде (фаза 12).
- `queries/advisory_unlock.sql` — `SELECT pg_advisory_unlock($1::bigint)`.
- `queries/reject_log_insert.sql` — `INSERT INTO reject_log (load_id, entity, payload, errors, severity) VALUES ($1, $2, $3, $4, $5)`.
- `queries/reject_log_select.sql` — фильтры `(load_id, entity, severity)` + cursor + limit.
- `queries/audit_access_insert.sql`
- `queries/audit_access_select.sql` (для будущего IT-read dashboard, в MVP — внутренний).

### Партиции (для cron pre-step фазы 12)

- `queries/partitions_create_month.sql` — функция или blueprint `CREATE TABLE IF NOT EXISTS <parent>_y<YYYY>m<MM> PARTITION OF <parent> FOR VALUES FROM (...) TO (...)`. Генерируется в коде с подстановкой имён.

### Тесты

- `internal/features/data_export/sqls/queries/embed_test.go`:
  - `TestEmbed_AllExpectedFilesPresent` — проверяет, что `FS` содержит все ожидаемые файлы (whitelist константа).
  - `TestEmbed_GetReturnsContent` — `Get("snapshot_select_current")` возвращает непустую строку.
  - `TestEmbed_GetUnknown_Panics` (или возвращает `""` + ошибку — единая контрактная семантика).

## Files to MODIFY

— нет.

## SQL/Migrations

— нет (только embed).

## Run after

```bash
make build
make test-unit
make lint
```

## Tests in this phase

- `TestEmbed_AllExpectedFilesPresent`
- `TestEmbed_GetReturnsContent`
- `TestEmbed_GetUnknown_Behavior`

## Definition of Done

- [ ] Все SQL-файлы созданы и embed-нуты.
- [ ] Каждый SELECT использует курсорную пагинацию (нет OFFSET).
- [ ] Запросы к partitioned-таблицам обязательно содержат фильтр по `event_date` (partition pruning).
- [ ] `snapshot_flip` — атомарный UPDATE одной строки.
- [ ] `advisory_lock_try` использует `pg_try_*` (не блокирующий).
- [ ] `make build` зелёный, `make lint` без ошибок.
- [ ] `make test-unit` зелёный.
- [ ] Коммит атомарный, сообщение `feat(data_export/sqls): ...`.
