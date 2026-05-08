-- UPSERT в supply_spec. Composite PK = (product_id, supplier_id, valid_from).
-- $1=product_id, $2=supplier_id, $3=pack_qty, $4=lead_time_days,
-- $5=min_order_qty, $6=multiple, $7=valid_from, $8=valid_to,
-- $9=load_id.
INSERT INTO supply_spec (product_id, supplier_id, pack_qty, lead_time_days,
                         min_order_qty, multiple, valid_from, valid_to, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (product_id, supplier_id, valid_from) DO UPDATE SET
    pack_qty       = EXCLUDED.pack_qty,
    lead_time_days = EXCLUDED.lead_time_days,
    min_order_qty  = EXCLUDED.min_order_qty,
    multiple       = EXCLUDED.multiple,
    valid_to       = EXCLUDED.valid_to,
    load_id        = EXCLUDED.load_id;
