-- $1 = load_id
SELECT load_id, started_at, committed_at, failed_at, status, failure_reason,
       source, lines_total, lines_failed, entity_stats
  FROM loads
 WHERE load_id = $1;
