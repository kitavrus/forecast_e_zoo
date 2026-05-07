-- Помечает зависшие etl_runs со status='running' старше $1 интервала
-- как 'aborted'. Запускается отдельным cron-job-ом на старте приложения
-- (см. ADR-025, design-sql.md §6).
-- Параметры: $1 interval (например '1 hour'::interval).
UPDATE marts.etl_runs
SET    status         = 'aborted',
       finished_at    = COALESCE(finished_at, now()),
       failure_reason = 'stale_timeout'
WHERE  status = 'running'
  AND  started_at < now() - $1::interval
RETURNING id;
