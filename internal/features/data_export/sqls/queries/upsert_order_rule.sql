-- UPSERT в order_rule. PK = id (text).
-- $1=id, $2=location_id, $3=product_id (NULLABLE),
-- $4=category_id (NULLABLE), $5=rule_type, $6=payload(jsonb),
-- $7=valid_from (date), $8=valid_to (date NULLABLE), $9=load_id (uuid).
INSERT INTO order_rule (id, location_id, product_id, category_id, rule_type,
                        payload, valid_from, valid_to, load_id)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
    location_id = EXCLUDED.location_id,
    product_id  = EXCLUDED.product_id,
    category_id = EXCLUDED.category_id,
    rule_type   = EXCLUDED.rule_type,
    payload     = EXCLUDED.payload,
    valid_from  = EXCLUDED.valid_from,
    valid_to    = EXCLUDED.valid_to,
    load_id     = EXCLUDED.load_id;
