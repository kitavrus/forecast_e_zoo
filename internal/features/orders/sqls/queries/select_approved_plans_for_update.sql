-- SELECT approved replenishment_plans для билда + lock рядов.
-- Игнорирует уже заблокированные SKIP LOCKED, чтобы parallel scheduler/manual run не зависал.
-- Параметры:
--   $1 = limit (int)
SELECT
    id,
    run_id,
    supplier_id,
    location_id,
    plan_date,
    total_qty::float8 AS total_qty,
    lines_count
FROM forecast.replenishment_plans
WHERE status = 'approved'
ORDER BY plan_date ASC, id ASC
LIMIT $1::int
FOR UPDATE SKIP LOCKED
