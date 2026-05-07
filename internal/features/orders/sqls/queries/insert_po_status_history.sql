-- Запись audit-перехода в po_status_history.
-- Параметры:
--   $1 = po_id (uuid)
--   $2 = from_status (text, nullable)
--   $3 = to_status (text)
--   $4 = reason (text, nullable)
--   $5 = changed_by (text, nullable)
INSERT INTO orders.po_status_history
    (po_id, from_status, to_status, reason, changed_by)
VALUES
    ($1::uuid, $2::text, $3::text, $4::text, $5::text)
