-- Append-only INSERT в stock_movement (партиционировано по event_date).
-- Composite PK = (event_date, id).
-- $1=id, $2=event_date, $3=event_time, $4=location_id, $5=product_id,
-- $6=movement_type, $7=qty, $8=ref_id, $9=payload(jsonb), $10=load_id.
INSERT INTO stock_movement (id, event_date, event_time, location_id, product_id,
                            movement_type, qty, ref_id, payload, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10);
