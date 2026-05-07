-- $1 = load_id
-- $2 = entity (text)
-- $3 = payload (jsonb) — исходный ERP DTO
-- $4 = errors (jsonb) — массив объектов {field, rule, message}
-- $5 = severity (text: 'error' | 'warn')  -- см. CHECK в migration 0001
INSERT INTO reject_log (load_id, entity, payload, errors, severity)
VALUES ($1, $2, $3, $4, $5);
