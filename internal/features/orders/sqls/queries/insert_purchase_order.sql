-- Создаёт purchase_order. Status фиксированный 'ready_to_send' для builder-а.
-- Параметры:
--   $1 = po_number (text)
--   $2 = plan_id (uuid)
--   $3 = supplier_id (text)
--   $4 = location_id (text)
--   $5 = total_qty (numeric)
--   $6 = total_amount (numeric, nullable)
--   $7 = currency (text)
--   $8 = delivery_date (date, nullable)
--   $9 = notes (text, nullable)
INSERT INTO orders.purchase_orders
    (po_number, plan_id, supplier_id, location_id,
     status, total_qty, total_amount, currency, delivery_date, notes)
VALUES
    ($1::text, $2::uuid, $3::text, $4::text,
     'ready_to_send', $5::numeric, $6::numeric, $7::text, $8::date, $9::text)
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
