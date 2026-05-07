-- SELECT plan по id.
-- Параметры:
--   $1 = plan_id (uuid)
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
WHERE id = $1::uuid
