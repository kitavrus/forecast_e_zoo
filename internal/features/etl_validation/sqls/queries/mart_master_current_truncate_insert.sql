-- INSERT текущего snapshot справочников в marts.mart_master_current.
-- Параметры: $1 etl_run_id (uuid), $2 source_load_id (uuid).
--
-- Имена PK-колонок берутся из json-полей DTO source-adapter:
--   stg_products.product_id, stg_locations.location_id, stg_suppliers.supplier_id.
--
-- ВАЖНО: TRUNCATE выполняется ОТДЕЛЬНЫМ Exec-вызовом из repository
-- (mart_master_current_truncate.sql); pgx.Tx.Exec не поддерживает multi-statement
-- query с $-параметрами (готовит prepared statement, который запрещает
-- multiple commands — SQLSTATE 42601).
INSERT INTO marts.mart_master_current (entity_type, entity_id, payload, etl_run_id, source_load_id)
SELECT 'product'::text             AS entity_type,
       p.product_id                AS entity_id,
       to_jsonb(p) - 'product_id'  AS payload,
       $1::uuid                    AS etl_run_id,
       $2::uuid                    AS source_load_id
FROM   pg_temp.stg_products p

UNION ALL

SELECT 'location'::text,
       l.location_id,
       to_jsonb(l) - 'location_id',
       $1::uuid,
       $2::uuid
FROM   pg_temp.stg_locations l

UNION ALL

SELECT 'supplier'::text,
       s.supplier_id,
       to_jsonb(s) - 'supplier_id',
       $1::uuid,
       $2::uuid
FROM   pg_temp.stg_suppliers s;
