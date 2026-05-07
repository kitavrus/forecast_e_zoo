-- $1 = current_load_id, $2 = after_pk (text "<event_date>|<id>"), $3 = limit
-- $4 = event_date_from, $5 = event_date_to
SELECT id, event_date, event_time, location_id, product_id, movement_type, qty,
       ref_id, payload, load_id
  FROM stock_movement
 WHERE load_id = $1
   AND event_date BETWEEN $4 AND $5
   AND (event_date::text || '|' || id::text) > $2
 ORDER BY event_date ASC, id ASC
 LIMIT $3;
