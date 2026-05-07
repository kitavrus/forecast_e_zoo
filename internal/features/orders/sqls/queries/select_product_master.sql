-- SELECT product unit_price из marts.mart_master_current.
-- payload — JSONB с полем unit_price.
-- Параметры:
--   $1 = product_id (text)
SELECT
    NULLIF(payload->>'unit_price', '')::numeric AS unit_price
FROM marts.mart_master_current
WHERE entity_type = 'product'
  AND entity_id   = $1::text
LIMIT 1
