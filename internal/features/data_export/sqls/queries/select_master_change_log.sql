-- $1 = current_load_id, $2 = after_pk (bigint id as text), $3 = limit
-- Опционально: фильтрация по entity осуществляется в repository через дополнительный AND.
SELECT id, entity, entity_pk, field, old_value, new_value, changed_at, load_id
  FROM master_change_log
 WHERE load_id = $1
   AND id > $2::bigint
 ORDER BY id ASC
 LIMIT $3;
