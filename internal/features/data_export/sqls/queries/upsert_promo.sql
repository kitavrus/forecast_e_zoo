-- UPSERT в promo. PK = id (text).
-- $1=id, $2=location_id, $3=product_id, $4=start_date, $5=end_date,
-- $6=discount_pct, $7=payload(jsonb), $8=load_id.
INSERT INTO promo (id, location_id, product_id, start_date, end_date,
                   discount_pct, payload, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
ON CONFLICT (id) DO UPDATE SET
    location_id  = EXCLUDED.location_id,
    product_id   = EXCLUDED.product_id,
    start_date   = EXCLUDED.start_date,
    end_date     = EXCLUDED.end_date,
    discount_pct = EXCLUDED.discount_pct,
    payload      = EXCLUDED.payload,
    updated_at   = now(),
    load_id      = EXCLUDED.load_id;
