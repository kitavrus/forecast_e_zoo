# Phase 04: Migrations 0002 — facts (partitioned by event_date)

**Цель:** создать `0002_facts_partitioned.up.sql` с **4 партиционированными по `event_date` таблицами фактов** (PG18 declarative RANGE-partitioning по месяцам). Создаются initial-партиции на текущий месяц + 2 будущих + 1 прошлый. Долгосрочное управление партициями (создание новых на лету) выносится либо на простой cron-task (фаза 12), либо обозначается как Q-NNN-открытое.

**Commit:** `feat(migrations): 0002 facts partitioned by event_date (RANGE monthly)`

---

## Files to CREATE

- `internal/features/data_export/sqls/migrations/0002_facts_partitioned.up.sql`:
  1. `receipt_line` (PARTITION BY RANGE (`event_date`)):
     - Колонки: `id bigint`, `receipt_id text`, `location_id text`, `product_id text`, `qty numeric`, `price numeric`, `event_time timestamptz`, `event_date date NOT NULL` (generated `STORED AS (event_time::date)` или передаётся явно — фиксируем явный передаваемый), `payload jsonb`, `load_id uuid`. PK `(event_date, id)` (PG18 требует partition key в PK).
     - Initial partitions: `receipt_line_y2026m04`, `receipt_line_y2026m05`, `receipt_line_y2026m06`, `receipt_line_y2026m07` (диапазоны по месяцам).
  2. `location_stock_snapshot` (PARTITION BY RANGE (`event_date`)):
     - `event_date date`, `location_id text`, `product_id text`, `qty_on_hand numeric`, `qty_reserved numeric`, `as_of timestamptz`, `load_id uuid`. PK `(event_date, location_id, product_id)`.
     - Initial: 4 партиции (как выше).
  3. `stock_movement` (PARTITION BY RANGE (`event_date`)):
     - `id bigint`, `event_date date`, `event_time timestamptz`, `location_id text`, `product_id text`, `movement_type text`, `qty numeric`, `ref_id text`, `payload jsonb`, `load_id uuid`. PK `(event_date, id)`.
     - Initial: 4 партиции.
  4. `supplier_stock_snapshot` (PARTITION BY RANGE (`event_date`)):
     - `event_date date`, `supplier_id text`, `product_id text`, `qty_available numeric`, `as_of timestamptz`, `load_id uuid`. PK `(event_date, supplier_id, product_id)`.
     - Initial: 4 партиции.
  - Индексы на каждой таблице (на каждой партиции автоматически унаследуются от родителя):
    - `receipt_line`: `(load_id)`, `(location_id, event_date)`, `(product_id, event_date)`.
    - `location_stock_snapshot`: `(load_id)`, `(location_id, event_date desc)`.
    - `stock_movement`: `(load_id)`, `(location_id, product_id, event_date)`.
    - `supplier_stock_snapshot`: `(load_id)`, `(supplier_id, event_date desc)`.

- `internal/features/data_export/sqls/migrations/0002_facts_partitioned.down.sql` — `DROP TABLE` 4-х партиционированных таблиц `CASCADE` (партиции удалятся каскадом).

- `internal/features/data_export/sqls/migrations/0002_test.go` (integration):
  - `TestMigration0002_PartitionedTablesExist` — проверяет `pg_partitioned_table` содержит все 4 имени.
  - `TestMigration0002_InitialPartitionsCreated` — проверяет `pg_inherits` даёт по 4 партиции на каждую родительскую.
  - `TestMigration0002_InsertRoutesToCorrectPartition` — `INSERT receipt_line(event_date='2026-05-15',...)` попадает в `receipt_line_y2026m05`.
  - `TestMigration0002_InsertOutsideRange_Fails` — `INSERT` с `event_date='2030-01-01'` падает с `no partition of relation`.

## Files to MODIFY

- `internal/features/data_export/sqls/migrations/embed.go` — без изменений (`*.sql` уже подхватит новые файлы).

## SQL/Migrations

> Замечание о partition lifecycle: Q-NNN-OPEN — кто создаёт месячные партиции после initial набора. **Решение MVP:** простая `CREATE PARTITION IF NOT EXISTS` SQL-функция, вызываемая из cron-tick фазы 12 (один-два раза в сутки). pg_partman пока не вводим (минимум зависимостей). Зафиксировать в `code-plan-phase-12-scheduler-admin-handlers.md` как обязательный pre-step cron-tick-а.

## Run after

```bash
docker-compose up -d postgres
make migrate-up
docker-compose exec postgres psql -U adapter -d source_adapter -c "\d+ receipt_line"
make test-integration
make migrate-down
```

## Tests in this phase

- `TestMigration0002_PartitionedTablesExist`
- `TestMigration0002_InitialPartitionsCreated`
- `TestMigration0002_InsertRoutesToCorrectPartition`
- `TestMigration0002_InsertOutsideRange_Fails`

## Definition of Done

- [ ] 4 партиционированные таблицы созданы с PARTITION BY RANGE по `event_date`.
- [ ] На каждую родительскую — initial-партиции по месяцам (минимум 4).
- [ ] PK содержит `event_date` (требование PG18 для partitioned).
- [ ] Все нужные индексы созданы на родителе (унаследованы партициями).
- [ ] Все integration-тесты зелёные.
- [ ] `migrate down` чистит всё.
- [ ] `make build` зелёный.
- [ ] Коммит атомарный, сообщение `feat(migrations): 0002 facts ...`.
