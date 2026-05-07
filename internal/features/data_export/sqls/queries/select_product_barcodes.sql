-- $1 = current_load_id (uuid)
-- $2 = after_pk (text) — sortable "<product_id>|<barcode>"
-- $3 = limit (int)
SELECT product_id, barcode, is_primary, load_id
  FROM product_barcodes
 WHERE load_id = $1
   AND (product_id || '|' || barcode) > $2
 ORDER BY product_id ASC, barcode ASC
 LIMIT $3;
