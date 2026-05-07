-- Cancel PO: переводит статус в 'cancelled' (idempotent fail если статус уже terminal/невалидный).
-- Параметры:
--   $1 = po_id (uuid)
--   $2 = reason (text, nullable)
UPDATE orders.purchase_orders
SET status = 'cancelled',
    cancel_reason = $2::text,
    updated_at = now()
WHERE id = $1::uuid
  AND status IN ('draft','ready_to_send','sent','confirmed_by_erp')
RETURNING
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
