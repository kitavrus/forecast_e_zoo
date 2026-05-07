-- SELECT status history одного PO.
-- Параметры:
--   $1 = po_id (uuid)
SELECT
    id,
    from_status,
    to_status,
    reason,
    changed_by,
    changed_at
FROM orders.po_status_history
WHERE po_id = $1::uuid
ORDER BY changed_at ASC
