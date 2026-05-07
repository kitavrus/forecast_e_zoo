-- Помечает run как committed с финальными счётчиками.
-- Параметры:
--   $1 = run_id (uuid)
--   $2 = forecasts_count (int)
--   $3 = lines_count (int)
--   $4 = plans_count (int)
UPDATE forecast.forecast_runs
SET status = 'committed',
    finished_at = now(),
    forecasts_count = $2::int,
    lines_count = $3::int,
    plans_count = $4::int
WHERE id = $1::uuid
RETURNING id
