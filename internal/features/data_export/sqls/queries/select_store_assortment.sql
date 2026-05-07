-- $1 = current_load_id, $2 = after_pk (text "<location_id>|<product_id>|<start_date>"), $3 = limit
SELECT location_id, product_id, start_date, end_date, is_active, updated_at, load_id
  FROM store_assortment
 WHERE load_id = $1
   AND (location_id || '|' || product_id || '|' || start_date::text) > $2
 ORDER BY location_id ASC, product_id ASC, start_date ASC
 LIMIT $3;
