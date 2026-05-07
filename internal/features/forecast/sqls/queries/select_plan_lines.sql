-- SELECT lines для конкретного plan'а: фильтр по run_id + supplier_id + location_id.
-- Параметры:
--   $1 = run_id (uuid)
--   $2 = supplier_id (text)
--   $3 = location_id (text)
SELECT
    id,
    product_id,
    location_id,
    supplier_id,
    current_stock::float8     AS current_stock,
    in_transit::float8        AS in_transit,
    daily_demand::float8      AS daily_demand,
    lead_time_days,
    safety_stock::float8      AS safety_stock,
    reorder_point::float8     AS reorder_point,
    target_stock::float8      AS target_stock,
    reorder_qty::float8       AS reorder_qty
FROM forecast.calculation_lines
WHERE run_id = $1::uuid
  AND supplier_id = $2::text
  AND location_id = $3::text
ORDER BY product_id ASC
