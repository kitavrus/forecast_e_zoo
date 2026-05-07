-- select_mart_calculation_input.sql
-- Cursor pagination по PK (product_id, location_id).
-- Параметры:
--   $1 = etl_run_id (uuid)
--   $2 = last_product_id  (text)
--   $3 = last_location_id (text)
--   $4 = limit (int)
SELECT
    product_id,
    location_id,
    on_hand,
    in_transit,
    safety_stock,
    forecast_qty_7d,
    forecast_qty_14d,
    rop,
    min_qty,
    max_qty,
    applicable_rule_id,
    applicable_rule_kind,
    formula,
    supplier_id,
    lead_time_days,
    etl_run_id,
    source_load_id,
    created_at
FROM marts.mart_calculation_input
WHERE etl_run_id = $1
  AND (product_id, location_id) > ($2, $3)
ORDER BY product_id, location_id
LIMIT $4;
