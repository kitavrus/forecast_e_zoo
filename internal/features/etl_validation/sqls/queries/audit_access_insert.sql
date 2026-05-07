INSERT INTO marts.audit_access (
    occurred_at, method, path, requester, role, status_code, request_id
) VALUES (
    now(), $1, $2, $3, $4, $5, $6
);
