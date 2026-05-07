-- List send_attempts с фильтрами и cursor pagination (id ASC).
-- Параметры:
--   $1 po_id (uuid, nullable), $2 supplier_id (text, nullable),
--   $3 status (text, nullable), $4 from (timestamptz, nullable),
--   $5 to (timestamptz, nullable), $6 cursor_id (uuid, nullable),
--   $7 limit (int).
SELECT
    id,
    po_id,
    supplier_id,
    channel_type,
    started_at,
    finished_at,
    status,
    http_status_code,
    error_message,
    retry_count,
    external_ref
FROM channels.send_attempts
WHERE ($1::uuid IS NULL OR po_id = $1::uuid)
  AND ($2::text IS NULL OR supplier_id = $2::text)
  AND ($3::text IS NULL OR status = $3::text)
  AND ($4::timestamptz IS NULL OR started_at >= $4::timestamptz)
  AND ($5::timestamptz IS NULL OR started_at < $5::timestamptz)
  AND ($6::uuid IS NULL OR id > $6::uuid)
ORDER BY id ASC
LIMIT $7::int
