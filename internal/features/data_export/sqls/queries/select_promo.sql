-- $1 = current_load_id, $2 = after_pk (text id), $3 = limit
SELECT id, location_id, product_id, start_date, end_date, discount_pct, payload,
       updated_at, load_id
  FROM promo
 WHERE load_id = $1
   AND id > $2
 ORDER BY id ASC
 LIMIT $3;
