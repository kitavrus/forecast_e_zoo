-- Append-only INSERT в supplier_stock_snapshot (партиционировано по event_date).
-- Composite PK = (event_date, supplier_id, product_id).
-- ON CONFLICT — на случай ретраев в пределах одного load (idempotent).
-- $1=event_date, $2=supplier_id, $3=product_id, $4=qty_available,
-- $5=as_of, $6=load_id.
INSERT INTO supplier_stock_snapshot (event_date, supplier_id, product_id,
                                     qty_available, as_of, load_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (event_date, supplier_id, product_id) DO UPDATE SET
    qty_available = EXCLUDED.qty_available,
    as_of         = EXCLUDED.as_of,
    load_id       = EXCLUDED.load_id;
