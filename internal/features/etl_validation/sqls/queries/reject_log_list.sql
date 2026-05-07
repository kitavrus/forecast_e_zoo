SELECT id, etl_run_id, entity, business_key, severity, rule_id, field, message, created_at
FROM   marts.reject_log
WHERE  ($1::uuid IS NULL OR etl_run_id = $1)
  AND  ($2::text IS NULL OR entity     = $2)
  AND  ($3::text IS NULL OR severity   = $3)
  AND  ($4::bigint IS NULL OR id < $4)
ORDER  BY id DESC
LIMIT  $5;
