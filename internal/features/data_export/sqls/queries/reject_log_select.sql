-- $1 = load_id (uuid)            — '00000000-0000-0000-0000-000000000000' = no filter
-- $2 = entity (text)              — '' = no filter
-- $3 = severity (text)            — '' = no filter
-- $4 = after_pk (bigint as text)  — id последней отданной строки, '' для начала
-- $5 = limit (int)
SELECT id, load_id, entity, payload, errors, severity, created_at
  FROM reject_log
 WHERE ($1 = '00000000-0000-0000-0000-000000000000'::uuid OR load_id = $1)
   AND ($2 = '' OR entity = $2)
   AND ($3 = '' OR severity = $3)
   AND ($4 = '' OR id > $4::bigint)
 ORDER BY id ASC
 LIMIT $5;
