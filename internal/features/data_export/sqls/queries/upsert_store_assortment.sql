-- UPSERT в store_assortment. Composite PK = (location_id, product_id).
-- $1=location_id, $2=product_id, $3=start_date, $4=end_date, $5=is_active,
-- $6=load_id.
INSERT INTO store_assortment (location_id, product_id, start_date, end_date,
                              is_active, load_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (location_id, product_id) DO UPDATE SET
    start_date = EXCLUDED.start_date,
    end_date   = EXCLUDED.end_date,
    is_active  = EXCLUDED.is_active,
    updated_at = now(),
    load_id    = EXCLUDED.load_id;
