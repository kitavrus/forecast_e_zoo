-- $1 = actor_role (''  = any)
-- $2 = path_prefix (''  = any)
-- $3 = since (timestamptz, '0001-01-01' = any)
-- $4 = after_pk (bigint as text, '' for start)
-- $5 = limit (int)
SELECT id, at, actor_role, actor_sub, method, path, status, trace_id
  FROM audit_access
 WHERE ($1 = '' OR actor_role = $1)
   AND ($2 = '' OR path LIKE $2 || '%')
   AND at >= $3
   AND ($4 = '' OR id > $4::bigint)
 ORDER BY id ASC
 LIMIT $5;
