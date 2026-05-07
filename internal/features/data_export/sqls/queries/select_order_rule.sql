-- $1 = current_load_id, $2 = after_pk (text id), $3 = limit
SELECT id, location_id, product_id, category_id, rule_type, payload,
       valid_from, valid_to, load_id
  FROM order_rule
 WHERE load_id = $1
   AND id > $2
 ORDER BY id ASC
 LIMIT $3;
