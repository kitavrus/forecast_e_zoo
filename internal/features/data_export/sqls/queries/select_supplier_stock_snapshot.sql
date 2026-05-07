-- $1 = current_load_id, $2 = after_pk (text "<event_date>|<supplier_id>|<product_id>"), $3 = limit
-- $4 = event_date_from, $5 = event_date_to
SELECT event_date, supplier_id, product_id, qty_available, as_of, load_id
  FROM supplier_stock_snapshot
 WHERE load_id = $1
   AND event_date BETWEEN $4 AND $5
   AND (event_date::text || '|' || supplier_id || '|' || product_id) > $2
 ORDER BY event_date ASC, supplier_id ASC, product_id ASC
 LIMIT $3;
