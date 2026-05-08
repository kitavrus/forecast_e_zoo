-- Fan-out supply_spec по локациям.
-- ETL ожидает rows с (product_id, supplier_id, location_id) — composite PK
-- в stg_supply_spec; репо-таблица supply_spec не хранит location_id (PK
-- product_id+supplier_id+valid_from), поэтому делаем CROSS JOIN с location.
-- $1 = current_load_id, $2 = after_pk (text "<product_id>|<supplier_id>|<location_id>"), $3 = limit
SELECT s.product_id,
       s.supplier_id,
       l.id AS location_id,
       s.pack_qty,
       s.lead_time_days,
       s.min_order_qty,
       s.multiple,
       s.valid_from,
       s.valid_to
  FROM supply_spec s
 CROSS JOIN location l
 WHERE s.load_id = $1
   AND l.load_id = $1
   AND (s.product_id || '|' || s.supplier_id || '|' || l.id) > $2
 ORDER BY s.product_id ASC, s.supplier_id ASC, l.id ASC
 LIMIT $3;
