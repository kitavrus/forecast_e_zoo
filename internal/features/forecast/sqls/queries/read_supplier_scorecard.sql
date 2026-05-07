-- Чтение mart_supplier_scorecard для актуальной недели → fallback lead_time.
-- Параметры:
--   $1 = from_date (date)
--   $2 = to_date (date)
SELECT
    supplier_id,
    AVG(lead_time_actual_avg)::float8 AS lead_time_actual_avg,
    AVG(fill_rate_avg)::float8        AS fill_rate_avg
FROM marts.mart_supplier_scorecard
WHERE week_start BETWEEN $1::date AND $2::date
GROUP BY supplier_id
