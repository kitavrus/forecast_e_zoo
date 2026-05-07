-- Перевод plan в статус 'converted' (после успешного INSERT PO).
-- Параметры:
--   $1 = plan_id (uuid)
UPDATE forecast.replenishment_plans
SET status = 'converted'
WHERE id = $1::uuid
  AND status = 'approved'
RETURNING id
