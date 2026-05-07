-- Bulk insert po_lines через UNNEST.
-- Параметры:
--   $1 = po_id (uuid)
--   $2 = product_ids (text[])
--   $3 = qtys (numeric[])
--   $4 = unit_prices (numeric[]; элементы могут быть NULL)
--   $5 = line_amounts (numeric[]; элементы могут быть NULL)
--   $6 = pricing_sources (text[])
INSERT INTO orders.po_lines
    (po_id, product_id, qty, unit_price, line_amount, pricing_source)
SELECT
    $1::uuid,
    p.product_id,
    p.qty,
    p.unit_price,
    p.line_amount,
    p.pricing_source
FROM UNNEST(
    $2::text[],
    $3::numeric[],
    $4::numeric[],
    $5::numeric[],
    $6::text[]
) AS p(product_id, qty, unit_price, line_amount, pricing_source)
