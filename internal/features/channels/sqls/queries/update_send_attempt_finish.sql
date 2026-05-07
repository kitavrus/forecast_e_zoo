-- Финализация попытки отправки.
-- Параметры:
--   $1 id, $2 started_at (для partition pruning),
--   $3 status, $4 http_status_code, $5 request_body, $6 response_body,
--   $7 error_message, $8 retry_count, $9 external_ref.
UPDATE channels.send_attempts
SET status           = $3::text,
    http_status_code = $4::int,
    request_body     = $5::text,
    response_body    = $6::text,
    error_message    = $7::text,
    retry_count      = $8::int,
    external_ref     = $9::text,
    finished_at      = now()
WHERE id = $1::uuid AND started_at = $2::timestamptz
