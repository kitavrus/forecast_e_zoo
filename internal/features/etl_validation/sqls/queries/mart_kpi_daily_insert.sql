-- Агрегация KPI по дням и локациям → marts.mart_kpi_daily (partitioned).
-- KPIs: revenue_total, transactions, returns_count, oos_pct (доля OOS-строк).
-- Параметры: $1 etl_run_id (uuid), $2 source_load_id (uuid).
INSERT INTO marts.mart_kpi_daily (
    as_of_date, location_id, kpi_name, kpi_value, kpi_unit,
    etl_run_id, source_load_id
)
SELECT as_of_date, location_id, kpi_name, kpi_value, kpi_unit, $1, $2
FROM (
    -- revenue_total
    SELECT rl.event_time::date                    AS as_of_date,
           rl.location_id                         AS location_id,
           'revenue_total'::text                  AS kpi_name,
           SUM(CASE WHEN rl.line_kind = 'sale'
                    THEN rl.qty * rl.unit_price_paid ELSE 0 END)::numeric AS kpi_value,
           'EUR'::text                            AS kpi_unit
    FROM   pg_temp.stg_receipt_line rl
    GROUP  BY rl.event_time::date, rl.location_id

    UNION ALL

    -- transactions
    SELECT rl.event_time::date,
           rl.location_id,
           'transactions',
           COUNT(DISTINCT rl.receipt_id)::numeric,
           'count'
    FROM   pg_temp.stg_receipt_line rl
    GROUP  BY rl.event_time::date, rl.location_id

    UNION ALL

    -- returns_count
    SELECT rl.event_time::date,
           rl.location_id,
           'returns_count',
           SUM(CASE WHEN rl.line_kind = 'return' THEN 1 ELSE 0 END)::numeric,
           'count'
    FROM   pg_temp.stg_receipt_line rl
    GROUP  BY rl.event_time::date, rl.location_id

    UNION ALL

    -- oos_pct: доля строк, где остаток на дату был 0.
    SELECT rl.event_time::date,
           rl.location_id,
           'oos_pct',
           (
               SUM(CASE WHEN COALESCE(soh.qty_on_hand, 0) = 0 THEN 1 ELSE 0 END)::numeric
               / NULLIF(COUNT(*), 0)::numeric
           ) * 100,
           'percent'
    FROM   pg_temp.stg_receipt_line rl
    LEFT JOIN pg_temp.stg_stock_on_hand soh
           ON soh.product_id  = rl.product_id
          AND soh.location_id = rl.location_id
          AND soh.as_of_date  = rl.event_time::date
    GROUP  BY rl.event_time::date, rl.location_id
) t
ON CONFLICT (location_id, kpi_name, as_of_date) DO UPDATE
   SET kpi_value      = EXCLUDED.kpi_value,
       etl_run_id     = EXCLUDED.etl_run_id,
       source_load_id = EXCLUDED.source_load_id,
       created_at     = now();
