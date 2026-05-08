-- Append-only INSERT в store_assortment_lifecycle_events.
-- id (bigserial) генерируется БД.
-- $1=location_id, $2=product_id, $3=event_type, $4=event_date,
-- $5=payload(jsonb), $6=load_id.
INSERT INTO store_assortment_lifecycle_events (location_id, product_id, event_type,
                                               event_date, payload, load_id)
VALUES ($1, $2, $3, $4, $5::jsonb, $6);
