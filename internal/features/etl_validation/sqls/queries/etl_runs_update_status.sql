UPDATE marts.etl_runs
SET    status         = $2,
       finished_at    = COALESCE($3, finished_at),
       committed_at   = COALESCE($4, committed_at),
       source_load_id = COALESCE($5, source_load_id),
       marts_summary  = COALESCE($6, marts_summary),
       failure_reason = COALESCE($7, failure_reason),
       lines_total    = COALESCE($8, lines_total),
       lines_failed   = COALESCE($9, lines_failed),
       updated_at     = now()
WHERE  id = $1;
