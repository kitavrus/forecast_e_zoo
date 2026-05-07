-- Чтение mart_calculation_input для последнего committed etl_run.
-- Параметры:
--   $1 = etl_run_id (uuid, nullable; если NULL → берётся latest committed)
SELECT
    ci.product_id,
    ci.location_id,
    ci.on_hand::float8       AS on_hand,
    ci.in_transit::float8    AS in_transit,
    ci.daily_demand::float8  AS daily_demand,
    ci.supplier_id,
    ci.lead_time_days,
    ci.safety_stock::float8  AS safety_stock,
    ci.min_qty::float8       AS min_qty,
    ci.max_qty::float8       AS max_qty
FROM marts.mart_calculation_input ci
WHERE ($1::uuid IS NULL OR ci.etl_run_id = $1::uuid)
