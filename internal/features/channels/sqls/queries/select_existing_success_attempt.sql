-- Idempotency: ищем существующий successful attempt для PO.
-- Возвращает 0 или 1 строку.
-- Параметр: $1 po_id.
SELECT id, external_ref, started_at
FROM channels.send_attempts
WHERE po_id = $1::uuid AND status = 'success'
ORDER BY started_at DESC
LIMIT 1
