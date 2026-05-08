-- UPSERT в supply_plan. PK = id (text).
-- $1=id, $2=location_id, $3=product_id, $4=supplier_id, $5=plan_date,
-- $6=qty, $7=payload(jsonb), $8=load_id.
INSERT INTO supply_plan (id, location_id, product_id, supplier_id, plan_date,
                         qty, payload, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
ON CONFLICT (id) DO UPDATE SET
    location_id = EXCLUDED.location_id,
    product_id  = EXCLUDED.product_id,
    supplier_id = EXCLUDED.supplier_id,
    plan_date   = EXCLUDED.plan_date,
    qty         = EXCLUDED.qty,
    payload     = EXCLUDED.payload,
    load_id     = EXCLUDED.load_id;
