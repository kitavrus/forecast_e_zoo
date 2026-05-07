-- $1 = load_id
-- $2 = failure_reason (text)
UPDATE loads
   SET status         = 'failed',
       failed_at      = now(),
       failure_reason = $2
 WHERE load_id = $1
   AND status  = 'running';
