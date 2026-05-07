-- Чтение mart_demand_history в окне [from, to] для построения SMA.
-- Параметры:
--   $1 = from_date (date)
--   $2 = to_date (date)
SELECT
    product_id,
    location_id,
    as_of_date,
    qty_sold::float8 AS qty_sold
FROM marts.mart_demand_history
WHERE as_of_date BETWEEN $1::date AND $2::date
ORDER BY product_id, location_id, as_of_date
