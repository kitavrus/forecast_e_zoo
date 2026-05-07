-- SELECT lines одного PO.
-- Параметры:
--   $1 = po_id (uuid)
SELECT
    id,
    product_id,
    qty::float8 AS qty,
    unit_price::float8 AS unit_price,
    line_amount::float8 AS line_amount,
    pricing_source,
    notes,
    created_at
FROM orders.po_lines
WHERE po_id = $1::uuid
ORDER BY product_id ASC
