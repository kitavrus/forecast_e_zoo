-- Fan-out order_rule по продуктам.
-- ETL ожидает строки с (rule_id, product_id, location_id) — без NULL по
-- product_id, иначе LEFT JOIN в mart_calculation_input не находит правило.
-- Если order_rule.product_id уже set — fan-out не делается (вернётся одна
-- строка). Если NULL — CROSS JOIN с products (соответствующей категории
-- если category_id != NULL, иначе со всеми).
-- $1 = current_load_id, $2 = after_pk (text "<rule_id>|<product_id>"), $3 = limit
WITH fanned AS (
    -- 1. Правила с явным product_id — без fanout.
    SELECT id          AS rule_id,
           location_id,
           product_id,
           rule_type,
           payload,
           valid_from,
           valid_to
      FROM order_rule
     WHERE load_id = $1
       AND product_id IS NOT NULL
    UNION ALL
    -- 2. Правила с category_id — fanout на products.category_id.
    SELECT r.id           AS rule_id,
           r.location_id,
           p.id           AS product_id,
           r.rule_type,
           r.payload,
           r.valid_from,
           r.valid_to
      FROM order_rule r
      JOIN products p ON p.category_id = r.category_id
     WHERE r.load_id = $1
       AND p.load_id = $1
       AND r.product_id IS NULL
       AND r.category_id IS NOT NULL
    UNION ALL
    -- 3. Location-wide rules (product_id NULL AND category_id NULL) — fanout на все products.
    SELECT r.id           AS rule_id,
           r.location_id,
           p.id           AS product_id,
           r.rule_type,
           r.payload,
           r.valid_from,
           r.valid_to
      FROM order_rule r
     CROSS JOIN products p
     WHERE r.load_id = $1
       AND p.load_id = $1
       AND r.product_id IS NULL
       AND r.category_id IS NULL
)
SELECT rule_id, location_id, product_id, rule_type, payload, valid_from, valid_to
  FROM fanned
 WHERE (rule_id || '|' || product_id) > $2
 ORDER BY rule_id ASC, product_id ASC
 LIMIT $3;
