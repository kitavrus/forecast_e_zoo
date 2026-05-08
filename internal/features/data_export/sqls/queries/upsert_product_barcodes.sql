-- UPSERT в product_barcodes. Composite PK = (product_id, barcode).
-- $1=product_id, $2=barcode, $3=is_primary, $4=load_id.
INSERT INTO product_barcodes (product_id, barcode, is_primary, load_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (product_id, barcode) DO UPDATE SET
    is_primary = EXCLUDED.is_primary,
    load_id    = EXCLUDED.load_id;
