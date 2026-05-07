-- Перевод PO в статус 'sent' с фиксацией канала.
-- Параметры: $1 po_id, $2 channel_type.
UPDATE orders.purchase_orders
SET status          = 'sent',
    sent_at         = now(),
    sent_to_channel = $2::text,
    updated_at      = now()
WHERE id = $1::uuid AND status = 'ready_to_send'
