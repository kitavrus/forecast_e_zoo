-- $1 = current_load_id (uuid)
-- $2 = after_pk (text "<event_date>|<id>")
-- $3 = limit (int)
-- $4 = event_date_from (date)
-- $5 = event_date_to (date)
-- partition pruning: фильтр по event_date BETWEEN $4 AND $5
SELECT id, receipt_id, location_id, product_id, qty, price,
       event_time, event_date, payload, load_id
  FROM receipt_line
 WHERE load_id = $1
   AND event_date BETWEEN $4 AND $5
   AND (event_date::text || '|' || id::text) > $2
 ORDER BY event_date ASC, id ASC
 LIMIT $3;
