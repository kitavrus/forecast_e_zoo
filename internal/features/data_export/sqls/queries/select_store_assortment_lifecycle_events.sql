-- $1 = current_load_id, $2 = after_pk (bigint id as text), $3 = limit
SELECT id, location_id, product_id, event_type, event_date, payload, load_id
  FROM store_assortment_lifecycle_events
 WHERE load_id = $1
   AND id > $2::bigint
 ORDER BY id ASC
 LIMIT $3;
