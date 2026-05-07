-- TRUNCATE marts.mart_master_current + INSERT текущего snapshot справочников.
-- Параметры: $1 etl_run_id (uuid), $2 source_load_id (uuid).
TRUNCATE TABLE marts.mart_master_current;

INSERT INTO marts.mart_master_current (entity_type, entity_id, payload, etl_run_id, source_load_id)
SELECT 'product'::text  AS entity_type,
       p.id             AS entity_id,
       to_jsonb(p) - 'id' AS payload,
       $1, $2
FROM   pg_temp.stg_products p

UNION ALL

SELECT 'location'::text,
       l.id,
       to_jsonb(l) - 'id',
       $1, $2
FROM   pg_temp.stg_locations l

UNION ALL

SELECT 'supplier'::text,
       s.id,
       to_jsonb(s) - 'id',
       $1, $2
FROM   pg_temp.stg_suppliers s;
