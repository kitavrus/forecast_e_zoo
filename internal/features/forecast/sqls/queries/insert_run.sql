-- Создаёт новый forecast run в статусе running.
-- Параметры:
--   $1 = horizon_days (int)
--   $2 = snapshot_etl_run_id (uuid, nullable)
INSERT INTO forecast.forecast_runs
    (status, horizon_days, snapshot_etl_run_id)
VALUES ('running', $1::int, $2::uuid)
RETURNING id, started_at, status, horizon_days, snapshot_etl_run_id, created_at
