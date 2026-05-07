# Phase 03 — Migrations 1001 — schema marts + 5 mart-таблиц

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Создать миграцию `1001_marts_schema` — `CREATE SCHEMA marts`, 5 mart-таблиц (`mart_demand_history` PARTITION BY RANGE month, `mart_calculation_input`, `mart_kpi_daily` PARTITION BY RANGE month, `mart_master_current`, `mart_supplier_scorecard`), PG-роль `mart_reader` + GRANT SELECT на `marts.*`, начальные партиции (текущий + следующий месяц).

## Commit

```
feat(etl/migrations): 1001 marts schema with 5 mart tables and mart_reader role
```

## Files to CREATE

- `internal/features/etl_validation/sqls/migrations/1001_marts_schema.up.sql`:
  - `CREATE SCHEMA IF NOT EXISTS marts AUTHORIZATION e_zoo;`
  - `CREATE TABLE marts.mart_demand_history (...) PARTITION BY RANGE (event_month);` (поля и индексы — см. design-sql.md §1).
  - `CREATE TABLE marts.mart_calculation_input (...);` (no partitions, маленькая).
  - `CREATE TABLE marts.mart_kpi_daily (...) PARTITION BY RANGE (kpi_month);`.
  - `CREATE TABLE marts.mart_master_current (...);`.
  - `CREATE TABLE marts.mart_supplier_scorecard (...);`.
  - `CREATE ROLE mart_reader NOLOGIN;` (idempotent — `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object THEN NULL; END $$;`).
  - `GRANT USAGE ON SCHEMA marts TO mart_reader;`
  - `GRANT SELECT ON ALL TABLES IN SCHEMA marts TO mart_reader;`
  - `ALTER DEFAULT PRIVILEGES IN SCHEMA marts GRANT SELECT ON TABLES TO mart_reader;`
  - Создание начальных партиций на текущий + следующий месяц для `mart_demand_history` и `mart_kpi_daily`.
- `internal/features/etl_validation/sqls/migrations/1001_marts_schema.down.sql`:
  - `DROP TABLE IF EXISTS marts.mart_supplier_scorecard;`
  - `DROP TABLE IF EXISTS marts.mart_master_current;`
  - `DROP TABLE IF EXISTS marts.mart_kpi_daily CASCADE;`
  - `DROP TABLE IF EXISTS marts.mart_calculation_input;`
  - `DROP TABLE IF EXISTS marts.mart_demand_history CASCADE;`
  - `DROP SCHEMA IF EXISTS marts CASCADE;`
  - Роль `mart_reader` НЕ удаляем (могут использовать другие БД-объекты).

## Files to MODIFY

- (нет)

## SQL / Migrations

Полный текст обеих миграций — `design-sql.md` §1. Ключевые поля:

- `mart_demand_history`: `etl_run_id`, `source_load_id`, `event_month` (PARTITION KEY), `product_id`, `location_id`, `qty`, `event_time`, индекс по `(event_month, product_id, location_id)`.
- `mart_calculation_input`: `product_id`, `location_id`, `on_hand`, `in_transit`, `safety_stock`, `applicable_rule_id`, `applicable_rule_kind`, `etl_run_id`, `source_load_id`, PRIMARY KEY (`product_id`, `location_id`).
- `mart_kpi_daily`: `kpi_month` PARTITION KEY, `kpi_date`, `metric`, `value`, `etl_run_id`, `source_load_id`.
- `mart_master_current`: snapshot текущего справочника products/locations/suppliers.
- `mart_supplier_scorecard`: ondemand-витрина поставщиков, обновляется через `/admin/marts/.../refresh`.

## Run after

```bash
make migrate-up-etl   # apply on dev DB
psql $ETL_DSN -c "\dn marts"
psql $ETL_DSN -c "\dt marts.*"
make migrate-down-etl
make migrate-up-etl
```

## Tests

- Integration test (фаза 06) включает прогон 1001 как часть suite setup (через `iofs` driver с `migrations` директории фичи).
- Manual smoke: на dev-DB up → проверить `\dn`, `\dt marts.*`, `\du mart_reader`, права.

## Definition of Done

- [ ] `1001_marts_schema.up.sql` и `.down.sql` созданы.
- [ ] Обе миграции идемпотентны (`IF EXISTS`/`IF NOT EXISTS` где применимо).
- [ ] `make migrate-up-etl` на чистой БД проходит без ошибок.
- [ ] `make migrate-down-etl` откатывает 1001 без оставшихся объектов в schema marts.
- [ ] Роль `mart_reader` создана + GRANT SELECT работает (`SET ROLE mart_reader; SELECT * FROM marts.mart_master_current LIMIT 0;`).
- [ ] Партиции на текущий + следующий месяц созданы для `mart_demand_history` и `mart_kpi_daily`.

## Зависимости

Использует базовый Postgres pool из Модуля 1; миграция применяется в отдельной schema, `data_export.*` не трогает.
