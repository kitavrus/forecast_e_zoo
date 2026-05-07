-- INSERT в marts.mart_calculation_input с приоритетом
-- order_rule > supply_spec (ADR-024).
-- Параметры: $1 etl_run_id (uuid), $2 source_load_id (uuid).
--
-- ВАЖНО: TRUNCATE выполняется ОТДЕЛЬНЫМ Exec-вызовом из repository
-- (mart_calculation_input_truncate.sql). pgx не поддерживает multi-statement
-- prepared query с $-параметрами (SQLSTATE 42601).
INSERT INTO marts.mart_calculation_input (
    product_id, location_id, on_hand, in_transit, safety_stock,
    forecast_horizon_days, daily_demand, min_qty, max_qty,
    applicable_rule_id, applicable_rule_kind, formula,
    supplier_id, lead_time_days,
    etl_run_id, source_load_id
)
WITH stock AS (
    SELECT product_id, location_id, qty_on_hand AS on_hand, qty_in_transit AS in_transit
    FROM   pg_temp.stg_stock_on_hand
),
rule_priority AS (
    -- order_rule имеет приоритет prio=1 (ADR-024).
    -- ВАЖНО: stg_order_rule.product_id может быть NULL (источник задаёт scope/scope_ref);
    -- COALESCE до '' гарантирует, что JOIN не теряет строки. is_active может быть
    -- NULL после ослабления NOT NULL — COALESCE до true сохраняет старое поведение.
    SELECT COALESCE(product_id, '')       AS product_id,
           COALESCE(location_id, '')      AS location_id,
           rule_id                        AS rule_id,
           'order_rule'::text             AS rule_kind,
           formula,
           min_qty, max_qty, safety_stock,
           forecast_horizon_days, daily_demand,
           supplier_id, lead_time_days,
           1 AS prio
    FROM   pg_temp.stg_order_rule
    WHERE  COALESCE(is_active, true)
    UNION ALL
    -- stg_supply_spec — composite-PK (supplier_id, product_id, location_id);
    -- собираем синтетический rule_id для applicable_rule_id из триплета.
    SELECT product_id,
           COALESCE(location_id, '')                                    AS location_id,
           supplier_id || ':' || product_id || ':' ||
               COALESCE(location_id, '')                                AS rule_id,
           'supply_spec'::text                                          AS rule_kind,
           NULL::text                                                   AS formula,
           min_qty, max_qty, safety_stock,
           NULL::int                                                    AS forecast_horizon_days,
           NULL::numeric                                                AS daily_demand,
           supplier_id, lead_time_days,
           2 AS prio
    FROM   pg_temp.stg_supply_spec
    WHERE  COALESCE(is_active, true)
),
chosen AS (
    SELECT DISTINCT ON (product_id, location_id) *
    FROM   rule_priority
    ORDER  BY product_id, location_id, prio
)
SELECT s.product_id,
       s.location_id,
       s.on_hand,
       s.in_transit,
       c.safety_stock,
       c.forecast_horizon_days,
       c.daily_demand,
       c.min_qty,
       c.max_qty,
       c.rule_id                                  AS applicable_rule_id,
       COALESCE(c.rule_kind, 'none')              AS applicable_rule_kind,
       c.formula,
       c.supplier_id,
       c.lead_time_days,
       $1                                         AS etl_run_id,
       $2                                         AS source_load_id
FROM   stock s
LEFT JOIN chosen c USING (product_id, location_id);
