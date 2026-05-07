-- SELECT calculation_lines для одного plan-а.
-- Plan однозначно идентифицируется через (run_id, supplier_id, location_id).
-- Параметры:
--   $1 = run_id (uuid)
--   $2 = supplier_id (text)
--   $3 = location_id (text)
SELECT
    product_id,
    location_id,
    supplier_id,
    reorder_qty::float8 AS reorder_qty
FROM forecast.calculation_lines
WHERE run_id = $1::uuid
  AND COALESCE(supplier_id, '') = $2::text
  AND location_id = $3::text
  AND reorder_qty > 0
ORDER BY product_id ASC
