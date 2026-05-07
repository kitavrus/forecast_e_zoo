-- Возвращает run по UUID.
-- Параметры:
--   $1 = run_id (uuid)
SELECT
    id,
    started_at,
    finished_at,
    status,
    horizon_days,
    snapshot_etl_run_id,
    forecasts_count,
    lines_count,
    plans_count,
    error_message,
    created_at
FROM forecast.forecast_runs
WHERE id = $1::uuid
