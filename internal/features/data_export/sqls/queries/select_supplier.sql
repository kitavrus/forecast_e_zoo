-- $1 = current_load_id, $2 = after_pk (text), $3 = limit
SELECT id, name, inn, kpp, updated_at, load_id
  FROM supplier
 WHERE load_id = $1
   AND id > $2
 ORDER BY id ASC
 LIMIT $3;
