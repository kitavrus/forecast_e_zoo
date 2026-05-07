-- select_mart_master_current.sql
-- Cursor pagination по PK (entity_type, entity_id).
-- Параметры:
--   $1 = etl_run_id (uuid)
--   $2 = last_entity_type (text)
--   $3 = last_entity_id   (text)
--   $4 = limit (int)
SELECT
    entity_type,
    entity_id,
    payload,
    etl_run_id,
    source_load_id,
    created_at
FROM marts.mart_master_current
WHERE etl_run_id = $1
  AND (entity_type, entity_id) > ($2, $3)
ORDER BY entity_type, entity_id
LIMIT $4;
