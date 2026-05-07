-- $1 = current_load_id (uuid)
-- $2 = after_pk (text)  — id последней отданной строки (или '')
-- $3 = limit (int)
SELECT id, sku, name, category_id, unit, pack_size, is_active, attributes, updated_at, load_id
  FROM products
 WHERE load_id = $1
   AND id > $2
 ORDER BY id ASC
 LIMIT $3;
