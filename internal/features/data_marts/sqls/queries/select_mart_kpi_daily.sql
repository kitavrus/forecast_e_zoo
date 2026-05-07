-- select_mart_kpi_daily.sql
-- Cursor pagination по PK (location_id, kpi_name, as_of_date).
-- Параметры:
--   $1 = etl_run_id (uuid)
--   $2 = last_location_id (text)
--   $3 = last_kpi_name    (text)
--   $4 = last_as_of_date  (date)
--   $5 = limit (int)
SELECT
    as_of_date,
    location_id,
    kpi_name,
    kpi_value,
    kpi_unit,
    etl_run_id,
    source_load_id,
    created_at
FROM marts.mart_kpi_daily
WHERE etl_run_id = $1
  AND (location_id, kpi_name, as_of_date) > ($2, $3, $4)
ORDER BY location_id, kpi_name, as_of_date
LIMIT $5;
