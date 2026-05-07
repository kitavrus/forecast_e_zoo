-- $1 = actor_role (text)
-- $2 = actor_sub  (text)
-- $3 = method     (text)
-- $4 = path       (text)
-- $5 = status     (int)
-- $6 = trace_id   (text)
INSERT INTO audit_access (actor_role, actor_sub, method, path, status, trace_id)
VALUES ($1, $2, $3, $4, $5, $6);
