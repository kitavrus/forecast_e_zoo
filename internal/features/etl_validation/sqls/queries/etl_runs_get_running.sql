SELECT id, started_at, finished_at, committed_at, status, kind, target_mart,
       source_load_id, parent_run_id, trigger, requester, marts_summary,
       failure_reason, lines_total, lines_failed, created_at, updated_at
FROM   marts.etl_runs
WHERE  status = 'running'
ORDER  BY started_at DESC
LIMIT  1;
