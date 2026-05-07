-- $1 = load_id (uuid)
-- $2 = source (text)
INSERT INTO loads (load_id, started_at, status, source)
VALUES ($1, now(), 'running', $2)
RETURNING load_id, started_at, status, source;
