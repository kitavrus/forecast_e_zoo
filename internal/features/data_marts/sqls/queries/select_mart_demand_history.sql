-- select_mart_demand_history.sql
-- Cursor pagination по PK (product_id, location_id, as_of_date).
-- Параметры:
--   $1 = etl_run_id (uuid)
--   $2 = last_product_id  (text, '' для начала)
--   $3 = last_location_id (text, '')
--   $4 = last_as_of_date  (date, '0001-01-01')
--   $5 = limit (int)
SELECT
    product_id,
    location_id,
    as_of_date,
    qty_sold,
    qty_returned,
    qty_promo_bonus,
    qty_gift,
    revenue_paid,
    discount_total,
    transactions_count,
    had_promo,
    promo_type,
    was_in_assortment,
    lifecycle_state_at_date,
    was_oos,
    etl_run_id,
    source_load_id,
    created_at
FROM marts.mart_demand_history
WHERE etl_run_id = $1
  AND (product_id, location_id, as_of_date) > ($2, $3, $4)
ORDER BY product_id, location_id, as_of_date
LIMIT $5;
