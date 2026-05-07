-- SELECT plans с фильтрами + cursor pagination.
-- Параметры:
--   $1 = supplier_id (text, nullable)
--   $2 = location_id (text, nullable)
--   $3 = plan_date (date, nullable)
--   $4 = status (text, nullable)
--   $5 = cursor_id (uuid, nullable)
--   $6 = limit (int)
SELECT
    id,
    run_id,
    supplier_id,
    location_id,
    plan_date,
    total_qty::float8 AS total_qty,
    lines_count,
    status,
    approved_at,
    approved_by,
    created_at
FROM forecast.replenishment_plans
WHERE ($1::text IS NULL OR supplier_id = $1::text)
  AND ($2::text IS NULL OR location_id = $2::text)
  AND ($3::date IS NULL OR plan_date = $3::date)
  AND ($4::text IS NULL OR status = $4::text)
  AND ($5::uuid IS NULL OR id > $5::uuid)
ORDER BY id ASC
LIMIT $6::int
