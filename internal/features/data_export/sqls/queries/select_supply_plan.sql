-- $1 = current_load_id, $2 = after_pk (text id), $3 = limit
SELECT id, location_id, product_id, supplier_id, plan_date, qty, payload, load_id
  FROM supply_plan
 WHERE load_id = $1
   AND id > $2
 ORDER BY id ASC
 LIMIT $3;
