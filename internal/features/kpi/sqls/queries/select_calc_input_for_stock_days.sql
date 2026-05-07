-- Чтение marts.mart_calculation_input для расчёта Stock Days.
SELECT
    product_id,
    location_id,
    COALESCE(on_hand, 0)::float8     AS on_hand,
    COALESCE(in_transit, 0)::float8  AS in_transit,
    daily_demand::float8             AS daily_demand,
    supplier_id
FROM marts.mart_calculation_input
