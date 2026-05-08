-- UPSERT в location_stock_snapshot. Партиционировано по event_date.
-- Composite PK = (event_date, location_id, product_id).
-- $1=event_date, $2=location_id, $3=product_id,
-- $4=qty_on_hand, $5=qty_reserved, $6=as_of, $7=load_id.
INSERT INTO location_stock_snapshot (event_date, location_id, product_id,
                                     qty_on_hand, qty_reserved, as_of, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (event_date, location_id, product_id) DO UPDATE SET
    qty_on_hand  = EXCLUDED.qty_on_hand,
    qty_reserved = EXCLUDED.qty_reserved,
    as_of        = EXCLUDED.as_of,
    load_id      = EXCLUDED.load_id;
