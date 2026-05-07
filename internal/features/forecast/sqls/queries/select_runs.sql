-- SELECT runs с фильтрами + cursor pagination по started_at DESC.
-- Параметры (NULL → пропуск фильтра):
--   $1 = status (text, nullable)
--   $2 = from_ts (timestamptz, nullable)
--   $3 = to_ts (timestamptz, nullable)
--   $4 = cursor_id (uuid, nullable)
--   $5 = limit (int)
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
WHERE ($1::text IS NULL OR status = $1::text)
  AND ($2::timestamptz IS NULL OR started_at >= $2::timestamptz)
  AND ($3::timestamptz IS NULL OR started_at <= $3::timestamptz)
  AND ($4::uuid IS NULL OR id < $4::uuid)
ORDER BY started_at DESC, id DESC
LIMIT $5::int
