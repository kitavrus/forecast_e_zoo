-- $1 = stale_after (text interval, e.g. '4 hours')
-- Помечаем все running-загрузки старше указанного интервала как aborted (stale).
-- Используется scheduler-ом на старте сервиса.
UPDATE loads
   SET status         = 'aborted',
       failed_at      = now(),
       failure_reason = 'stale'
 WHERE status     = 'running'
   AND started_at < now() - $1::interval;
