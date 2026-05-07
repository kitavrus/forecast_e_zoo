# Phase 03: Migrations 0001 — master + service tables

**Цель:** создать первый migration-файл `0001_master_and_service.up.sql` (+ `.down.sql`) golang-migrate/v4 с **17 не-партиционированными** таблицами: master-данными, lifecycle-event журналом, служебными `loads` / `snapshot_pointer` / `reject_log` / `entity_checkpoint` / `audit_access`. Партиционированные факты выносятся в фазу 04.

**Commit:** `feat(migrations): 0001 master + service tables (17 tables)`

---

## Files to CREATE

- `internal/features/data_export/sqls/migrations/0001_master_and_service.up.sql` — DDL:
  1. `loads` (`load_id uuid PK`, `started_at`, `committed_at`, `failed_at`, `status` enum-text running|committed|failed|aborted, `failed_reason text`, `source text`, `lines_total bigint`, `lines_failed bigint`, `entity_stats jsonb`, индексы: `(status, started_at)`, `(source, started_at desc)`).
  2. `snapshot_pointer` (`id smallint PK = 1`, `current_load_id uuid NULL`, `previous_load_id uuid NULL`, `committed_at timestamptz`, FK на `loads`). Single-row table (см. [design-sql.md](design-sql.md) §1, ADR-008).
  3. `reject_log` (`id bigserial PK`, `load_id uuid FK`, `entity text`, `payload jsonb`, `errors jsonb`, `severity text`, `created_at timestamptz default now()`, индекс `(load_id, entity)`).
  4. `entity_checkpoint` (`entity text PK`, `last_load_id uuid`, `last_committed_at timestamptz`).
  5. `audit_access` (`id bigserial PK`, `at timestamptz default now()`, `actor_role text`, `actor_sub text`, `method text`, `path text`, `status int`, `trace_id text`, индекс `(at desc)`, ретеншн-комментарий 365d через TODO).
  6. `category` (`id text PK`, `parent_id text NULL`, `name text NOT NULL`, `path ltree NULL`, `updated_at`, `load_id`).
  7. `location` (`id text PK`, `type text`, `name text`, `region text`, `address text`, `lat numeric`, `lon numeric`, `updated_at`, `load_id`).
  8. `supplier` (`id text PK`, `name text`, `inn text`, `kpp text`, `updated_at`, `load_id`).
  9. `products` (`id text PK`, `sku text UNIQUE`, `name text`, `category_id text FK`, `unit text`, `pack_size numeric`, `is_active bool`, `attributes jsonb`, `updated_at`, `load_id`).
  10. `product_barcodes` (`product_id text FK`, `barcode text`, `is_primary bool`, PK `(product_id, barcode)`, индекс `(barcode)`).
  11. `store_assortment` (`location_id text`, `product_id text`, `start_date date`, `end_date date NULL`, `is_active bool`, `updated_at`, `load_id`, PK `(location_id, product_id)`).
  12. `store_assortment_lifecycle_events` (`id bigserial PK`, `location_id text`, `product_id text`, `event_type text` enum start|stop|promo, `event_date date`, `payload jsonb`, `load_id`, индекс `(location_id, product_id, event_date desc)`).
  13. `supply_spec` (`product_id text`, `supplier_id text`, `pack_qty numeric`, `lead_time_days int`, `min_order_qty numeric`, `multiple numeric`, `valid_from date`, `valid_to date NULL`, PK `(product_id, supplier_id, valid_from)`).
  14. `promo` (`id text PK`, `location_id text`, `product_id text`, `start_date`, `end_date`, `discount_pct numeric`, `payload jsonb`, `updated_at`, `load_id`).
  15. `order_rule` (`id text PK`, `location_id text`, `product_id text NULL`, `category_id text NULL`, `rule_type text`, `payload jsonb`, `valid_from date`, `valid_to date NULL`, `load_id`).
  16. `supply_plan` (`id text PK`, `location_id text`, `product_id text`, `supplier_id text`, `plan_date date`, `qty numeric`, `payload jsonb`, `load_id`).
  17. `master_change_log` (`id bigserial PK`, `entity text`, `entity_pk jsonb`, `field text`, `old_value jsonb`, `new_value jsonb`, `changed_at timestamptz`, `load_id uuid`, индекс `(entity, changed_at desc)`).

- `internal/features/data_export/sqls/migrations/0001_master_and_service.down.sql` — `DROP TABLE ... CASCADE` в обратном порядке.

- `internal/features/data_export/sqls/migrations/embed.go` — пустой файл с `package migrations` + `//go:embed *.sql` `var FS embed.FS` (используется в фазе 06/15).

- `internal/features/data_export/sqls/migrations/0001_test.go` — integration test (tag `integration`) через dockertest:
  - `TestMigration0001_Up_DropTables` — поднимает PG18, прогоняет `up`, проверяет `information_schema.tables` содержит все 17 имён.
  - `TestMigration0001_Down_DropsAll` — `up` → `down` → ни одной таблицы из списка.
  - `TestSnapshotPointerSingleRow` — после `up` есть seed-строка `id=1` (если решено seed-ить, иначе тест проверяет UNIQUE constraint).
  - `TestLoadsStatusCheck` — INSERT с невалидным `status` падает.

## Files to MODIFY

- `Makefile` — таргет `migrate-up` указывает на `internal/features/data_export/sqls/migrations`.
- `docker-compose.yml` — `migrate` сервис volume на этот путь.
- `go.mod` / `go.sum` — `github.com/ory/dockertest/v3` (если ещё нет — добавить, понадобится фазе 08), `github.com/golang-migrate/migrate/v4/database/postgres`, `github.com/golang-migrate/migrate/v4/source/file`.

## Run after

```bash
docker-compose up -d postgres
make migrate-up
docker-compose exec postgres psql -U adapter -d source_adapter -c "\dt"
make test-integration
make migrate-down
```

## Tests in this phase

- `TestMigration0001_Up_DropTables`
- `TestMigration0001_Down_DropsAll`
- `TestSnapshotPointerSingleRow`
- `TestLoadsStatusCheck`

## Definition of Done

- [ ] Файл `0001_master_and_service.up.sql` создаёт ровно 17 таблиц.
- [ ] `migrate up` идемпотентен.
- [ ] `migrate down` чистит всё созданное в `up`.
- [ ] FK-связи валидны (categories → products, supplier → supply_spec и т.д.).
- [ ] Все integration-тесты зелёные (`make test-integration -tags=integration`).
- [ ] `make build` зелёный.
- [ ] Коммит атомарный, сообщение `feat(migrations): 0001 master ...`.
