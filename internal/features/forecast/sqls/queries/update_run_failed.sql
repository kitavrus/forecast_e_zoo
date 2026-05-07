-- Помечает run как failed с error_message.
-- Параметры:
--   $1 = run_id (uuid)
--   $2 = error_message (text)
UPDATE forecast.forecast_runs
SET status = 'failed',
    finished_at = now(),
    error_message = $2::text
WHERE id = $1::uuid
RETURNING id
