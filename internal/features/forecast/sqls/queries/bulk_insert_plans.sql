-- Bulk INSERT replenishment_plans через UNNEST.
-- Параметры:
--   $1 = run_id (uuid)
--   $2 = supplier_ids (text[])
--   $3 = location_ids (text[])
--   $4 = plan_dates (date[])
--   $5 = total_qtys (numeric[])
--   $6 = lines_counts (int[])
INSERT INTO forecast.replenishment_plans
    (run_id, supplier_id, location_id, plan_date, total_qty, lines_count, status)
SELECT
    $1::uuid,
    p.supplier_id,
    p.location_id,
    p.plan_date,
    p.total_qty,
    p.lines_count,
    'draft'
FROM UNNEST(
    $2::text[], $3::text[], $4::date[], $5::numeric[], $6::int[]
) AS p(supplier_id, location_id, plan_date, total_qty, lines_count)
