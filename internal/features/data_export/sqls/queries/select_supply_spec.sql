-- $1 = current_load_id, $2 = after_pk (text "<product_id>|<supplier_id>|<valid_from>"), $3 = limit
SELECT product_id, supplier_id, pack_qty, lead_time_days, min_order_qty, multiple,
       valid_from, valid_to, load_id
  FROM supply_spec
 WHERE load_id = $1
   AND (product_id || '|' || supplier_id || '|' || valid_from::text) > $2
 ORDER BY product_id ASC, supplier_id ASC, valid_from ASC
 LIMIT $3;
