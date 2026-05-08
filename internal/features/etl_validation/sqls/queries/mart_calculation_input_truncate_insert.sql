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
    -- Dedup на стороне stock: source-adapter /v1/location_stock_snapshot отдаёт
    -- snapshot-историю (после pagination fix — все исторические события), поэтому
    -- одна пара (product_id, location_id) приходит несколько раз. PK
    -- mart_calculation_input — (product_id, location_id), поэтому берём ОДНУ
    -- строку детерминистично: предпочитаем самый высокий qty_on_hand
    -- (proxy для последнего достоверного snapshot'а), при равенстве — больший
    -- in_transit.
    SELECT DISTINCT ON (product_id, location_id)
           product_id,
           location_id,
           qty_on_hand    AS on_hand,
           qty_in_transit AS in_transit
    FROM   pg_temp.stg_stock_on_hand
    ORDER  BY product_id, location_id,
              qty_on_hand DESC NULLS LAST,
              qty_in_transit DESC NULLS LAST
),
-- supply_spec_dedup: одна строка на пару (product_id, location_id) с
-- детерминистичным выбором поставщика. Приоритет — наименьший lead_time_days
-- (самый быстрый поставщик), при равенстве — MIN(supplier_id) для стабильного
-- порядка. Используется и в rule_priority (для prio=2), и в supplier_fallback
-- (для случая, когда order_rule выигрывает по prio=1, но не отдаёт supplier).
supply_spec_dedup AS (
    SELECT DISTINCT ON (product_id, location_id)
           supplier_id,
           product_id,
           COALESCE(location_id, '') AS location_id,
           lead_time_days,
           min_qty,
           max_qty,
           safety_stock
    FROM   pg_temp.stg_supply_spec
    WHERE  COALESCE(is_active, true)
    ORDER  BY product_id,
              COALESCE(location_id, ''),
              lead_time_days ASC NULLS LAST,
              supplier_id ASC
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
    -- supply_spec_dedup гарантирует ОДНУ строку на (product_id, location_id);
    -- собираем синтетический rule_id для applicable_rule_id из триплета.
    SELECT product_id,
           location_id,
           supplier_id || ':' || product_id || ':' || location_id AS rule_id,
           'supply_spec'::text                                    AS rule_kind,
           NULL::text                                             AS formula,
           min_qty, max_qty, safety_stock,
           NULL::int                                              AS forecast_horizon_days,
           NULL::numeric                                          AS daily_demand,
           supplier_id, lead_time_days,
           2 AS prio
    FROM   supply_spec_dedup
),
chosen AS (
    SELECT DISTINCT ON (product_id, location_id) *
    FROM   rule_priority
    ORDER  BY product_id, location_id, prio
),
-- supplier_fallback нужен, когда order_rule выигрывает приоритет, но не отдаёт
-- supplier_id/lead_time_days. Без этого fallback'а forecast не может построить
-- replenishment_plans (требуется supplier_id для сборки PO). Берём из
-- уже дедуплицированного supply_spec_dedup.
supplier_fallback AS (
    SELECT product_id, location_id, supplier_id, lead_time_days,
           min_qty       AS ss_min_qty,
           max_qty       AS ss_max_qty,
           safety_stock  AS ss_safety_stock
    FROM   supply_spec_dedup
)
SELECT s.product_id,
       s.location_id,
       s.on_hand,
       s.in_transit,
       COALESCE(c.safety_stock, sf.ss_safety_stock)         AS safety_stock,
       c.forecast_horizon_days,
       c.daily_demand,
       COALESCE(c.min_qty, sf.ss_min_qty)                   AS min_qty,
       COALESCE(c.max_qty, sf.ss_max_qty)                   AS max_qty,
       c.rule_id                                            AS applicable_rule_id,
       COALESCE(c.rule_kind, 'none')                        AS applicable_rule_kind,
       c.formula,
       COALESCE(c.supplier_id, sf.supplier_id)              AS supplier_id,
       COALESCE(c.lead_time_days, sf.lead_time_days)        AS lead_time_days,
       $1                                                   AS etl_run_id,
       $2                                                   AS source_load_id
FROM   stock s
LEFT JOIN chosen c USING (product_id, location_id)
LEFT JOIN supplier_fallback sf USING (product_id, location_id);
