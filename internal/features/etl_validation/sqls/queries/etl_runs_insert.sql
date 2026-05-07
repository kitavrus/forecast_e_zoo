INSERT INTO marts.etl_runs (
    id, started_at, status, kind, target_mart, source_load_id,
    parent_run_id, trigger, requester, marts_summary,
    failure_reason, lines_total, lines_failed
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE($10, '{}'::jsonb), $11, $12, $13
);
