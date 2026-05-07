-- Атомарный flip snapshot pointer. Вызывается ВНУТРИ transaction loader-а
-- (см. фаза 10), после успешного UPSERT всех сущностей.
-- $1 = new_load_id (uuid).
UPDATE snapshot_pointer
   SET previous_load_id = current_load_id,
       current_load_id  = $1,
       committed_at     = now()
 WHERE id = 1
RETURNING current_load_id, previous_load_id, committed_at;
