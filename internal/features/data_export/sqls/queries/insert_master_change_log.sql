-- Append-only INSERT в master_change_log.
-- id (bigserial) генерируется БД.
-- $1=entity, $2=entity_pk(jsonb), $3=field, $4=old_value(jsonb),
-- $5=new_value(jsonb), $6=changed_at, $7=load_id.
INSERT INTO master_change_log (entity, entity_pk, field, old_value, new_value,
                               changed_at, load_id)
VALUES ($1, $2::jsonb, $3, $4::jsonb, $5::jsonb, COALESCE($6, now()), $7);
