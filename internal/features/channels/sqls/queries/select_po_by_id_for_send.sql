-- Получение PO для отправки (без lock, т.к. retry path).
-- Параметр: $1 po_id.
SELECT
    id,
    po_number,
    supplier_id,
    location_id,
    status,
    total_qty::float8 AS total_qty,
    currency,
    created_at
FROM orders.purchase_orders
WHERE id = $1::uuid
LIMIT 1
