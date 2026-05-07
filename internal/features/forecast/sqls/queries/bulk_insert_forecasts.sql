-- Bulk INSERT forecasts через UNNEST массивов.
-- Параметры (parallel arrays):
--   $1 = run_id (uuid)
--   $2 = product_ids (text[])
--   $3 = location_ids (text[])
--   $4 = forecast_dates (date[])
--   $5 = forecast_qtys (numeric[])
--   $6 = lower_bounds (numeric[])
--   $7 = upper_bounds (numeric[])
--   $8 = model_name (text)
--   $9 = confidence (numeric)
INSERT INTO forecast.forecasts
    (run_id, product_id, location_id, forecast_date, forecast_qty, lower_bound, upper_bound, model_name, confidence)
SELECT
    $1::uuid,
    p.product_id,
    p.location_id,
    p.forecast_date,
    p.forecast_qty,
    p.lower_bound,
    p.upper_bound,
    $8::text,
    $9::numeric
FROM UNNEST(
    $2::text[], $3::text[], $4::date[], $5::numeric[], $6::numeric[], $7::numeric[]
) AS p(product_id, location_id, forecast_date, forecast_qty, lower_bound, upper_bound)
