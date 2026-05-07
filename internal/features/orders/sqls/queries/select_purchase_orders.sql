-- SELECT PO с фильтрами + cursor pagination (id ASC).
-- Параметры:
--   $1 = status (text, nullable)
--   $2 = supplier_id (text, nullable)
--   $3 = plan_id (uuid, nullable)
--   $4 = from (timestamptz, nullable)
--   $5 = to (timestamptz, nullable)
--   $6 = cursor_id (uuid, nullable)
--   $7 = limit (int)
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
WHERE ($1::text IS NULL OR status = $1::text)
  AND ($2::text IS NULL OR supplier_id = $2::text)
  AND ($3::uuid IS NULL OR plan_id = $3::uuid)
  AND ($4::timestamptz IS NULL OR created_at >= $4::timestamptz)
  AND ($5::timestamptz IS NULL OR created_at < $5::timestamptz)
  AND ($6::uuid IS NULL OR id > $6::uuid)
ORDER BY id ASC
LIMIT $7::int
