-- $1 = current_load_id, $2 = after_pk (text "<event_date>|<location_id>|<product_id>"), $3 = limit
-- $4 = event_date_from, $5 = event_date_to
SELECT event_date, location_id, product_id, qty_on_hand, qty_reserved, as_of, load_id
  FROM location_stock_snapshot
 WHERE load_id = $1
   AND event_date BETWEEN $4 AND $5
   AND (event_date::text || '|' || location_id || '|' || product_id) > $2
 ORDER BY event_date ASC, location_id ASC, product_id ASC
 LIMIT $3;
