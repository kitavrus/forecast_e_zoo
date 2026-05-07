-- SELECT одного PO по id.
-- Параметры:
--   $1 = po_id (uuid)
SELECT
    id,
    po_number,
    plan_id,
    supplier_id,
    location_id,
    status,
    total_qty::float8 AS total_qty,
    total_amount::float8 AS total_amount,
    currency,
    delivery_date,
    notes,
    sent_at,
    sent_to_channel,
    cancel_reason,
    created_at,
    updated_at
FROM orders.purchase_orders
WHERE id = $1::uuid
LIMIT 1
