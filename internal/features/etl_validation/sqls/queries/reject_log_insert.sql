INSERT INTO marts.reject_log (
    etl_run_id, entity, business_key, severity, rule_id, field, message
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
);
