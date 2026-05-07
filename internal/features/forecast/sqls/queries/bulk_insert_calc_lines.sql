-- Bulk INSERT calculation_lines через UNNEST массивов.
-- Параметры (parallel arrays):
--   $1 = run_id (uuid)
--   $2 = product_ids (text[])
--   $3 = location_ids (text[])
--   $4 = supplier_ids (text[])
--   $5 = current_stocks (numeric[])
--   $6 = in_transits (numeric[])
--   $7 = daily_demands (numeric[])
--   $8 = lead_time_days (int[])
--   $9 = safety_stocks (numeric[])
--  $10 = reorder_points (numeric[])
--  $11 = target_stocks (numeric[])
--  $12 = reorder_qtys (numeric[])
INSERT INTO forecast.calculation_lines
    (run_id, product_id, location_id, supplier_id, current_stock, in_transit,
     daily_demand, lead_time_days, safety_stock, reorder_point, target_stock, reorder_qty)
SELECT
    $1::uuid,
    p.product_id,
    p.location_id,
    NULLIF(p.supplier_id, ''),
    p.current_stock,
    p.in_transit,
    p.daily_demand,
    p.lead_time_days,
    p.safety_stock,
    p.reorder_point,
    p.target_stock,
    p.reorder_qty
FROM UNNEST(
    $2::text[], $3::text[], $4::text[], $5::numeric[], $6::numeric[],
    $7::numeric[], $8::int[], $9::numeric[], $10::numeric[], $11::numeric[], $12::numeric[]
) AS p(product_id, location_id, supplier_id, current_stock, in_transit,
       daily_demand, lead_time_days, safety_stock, reorder_point, target_stock, reorder_qty)
