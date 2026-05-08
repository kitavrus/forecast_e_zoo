-- Rollback: убираем load_id из product_barcodes.
DROP INDEX IF EXISTS idx_product_barcodes_load_id;
ALTER TABLE product_barcodes DROP COLUMN IF EXISTS load_id;
