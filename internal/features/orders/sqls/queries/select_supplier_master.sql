-- SELECT supplier из marts.mart_master_current.
-- payload — JSONB с полями {currency_code, lead_time_days, default_unit_price}.
-- Возвращает (currency_code, lead_time_days, default_unit_price).
-- Параметры:
--   $1 = supplier_id (text)
SELECT
    COALESCE(payload->>'currency_code', '')                AS currency_code,
    COALESCE((payload->>'lead_time_days')::int, 0)         AS lead_time_days,
    NULLIF(payload->>'default_unit_price', '')::numeric    AS default_unit_price
FROM marts.mart_master_current
WHERE entity_type = 'supplier'
  AND entity_id   = $1::text
LIMIT 1
