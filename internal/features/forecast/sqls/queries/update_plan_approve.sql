-- Approve plan: status draft → approved (idempotent fail если status != draft).
-- Параметры:
--   $1 = plan_id (uuid)
--   $2 = approved_by (text)
UPDATE forecast.replenishment_plans
SET status = 'approved',
    approved_at = now(),
    approved_by = $2::text
WHERE id = $1::uuid
  AND status = 'draft'
RETURNING
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
