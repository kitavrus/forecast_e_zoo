-- Получить attempt по id (включая request/response bodies).
-- Параметр: $1 id.
SELECT
    id,
    po_id,
    supplier_id,
    channel_type,
    started_at,
    finished_at,
    status,
    http_status_code,
    request_body,
    response_body,
    error_message,
    retry_count,
    external_ref
FROM channels.send_attempts
WHERE id = $1::uuid
LIMIT 1
