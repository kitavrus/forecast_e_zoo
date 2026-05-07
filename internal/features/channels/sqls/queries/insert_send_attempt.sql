-- Insert новой попытки отправки (status='pending').
-- Параметры:
--   $1 po_id, $2 supplier_id, $3 channel_type, $4 status, $5 retry_count.
INSERT INTO channels.send_attempts (
    po_id, supplier_id, channel_type, status, retry_count, started_at
) VALUES ($1::uuid, $2::text, $3::text, $4::text, $5::int, now())
RETURNING id, started_at
