-- Lock и забрать ready_to_send PO для отправки в каналы.
-- Параметры: $1 = limit (int).
SELECT
    id,
    po_number,
    supplier_id,
    location_id,
    total_qty::float8 AS total_qty,
    currency,
    created_at
FROM orders.purchase_orders
WHERE status = 'ready_to_send'
ORDER BY created_at ASC
LIMIT $1::int
FOR UPDATE SKIP LOCKED
