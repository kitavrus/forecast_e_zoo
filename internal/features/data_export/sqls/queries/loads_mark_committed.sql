-- $1 = load_id
-- $2 = lines_total (bigint)
-- $3 = lines_failed (bigint)
-- $4 = entity_stats (jsonb)
UPDATE loads
   SET status        = 'committed',
       committed_at  = now(),
       lines_total   = $2,
       lines_failed  = $3,
       entity_stats  = $4
 WHERE load_id = $1
   AND status  = 'running';
