-- Аггрегация marts.mart_demand_history для OSA.
-- $1 = lookback_from (DATE), $2 = lookback_to (DATE) inclusive.
-- Возвращает (product_id, location_id, days_observed, days_oos).
SELECT
    product_id,
    location_id,
    COUNT(*)::int                                            AS days_observed,
    COALESCE(SUM(CASE WHEN was_oos THEN 1 ELSE 0 END), 0)::int AS days_oos
FROM marts.mart_demand_history
WHERE as_of_date BETWEEN $1::date AND $2::date
GROUP BY product_id, location_id
