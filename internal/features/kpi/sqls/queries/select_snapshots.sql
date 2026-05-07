-- SELECT с фильтрами + cursor pagination.
-- Параметры (NULL → пропуск фильтра):
--   $1 = as_of_date (DATE, nullable)
--   $2 = kpi_name (TEXT, nullable)
--   $3 = scope_type (TEXT, nullable)
--   $4 = scope_id (TEXT, nullable)
--   $5 = cursor_id (UUID, nullable; вернёт строки с id > cursor)
--   $6 = limit (INT)
SELECT
    id,
    as_of_date,
    kpi_name,
    scope_type,
    scope_id,
    value::float8       AS value,
    calibration_id,
    computed_at,
    etl_run_id
FROM kpi.kpi_snapshots
WHERE ($1::date IS NULL OR as_of_date = $1::date)
  AND ($2::text IS NULL OR kpi_name   = $2::text)
  AND ($3::text IS NULL OR scope_type = $3::text)
  AND ($4::text IS NULL OR scope_id   = $4::text)
  AND ($5::uuid IS NULL OR id > $5::uuid)
ORDER BY id ASC
LIMIT $6::int
