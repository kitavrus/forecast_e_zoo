-- Агрегация stg_receipt_line + LEFT JOIN stg_promo / stg_store_assortment / stg_stock_on_hand
-- → marts.mart_demand_history с upsert по PK (product_id, location_id, as_of_date).
-- Параметры: $1 etl_run_id (uuid), $2 source_load_id (uuid).
INSERT INTO marts.mart_demand_history (
    as_of_date, location_id, product_id,
    qty_sold, qty_returned, qty_promo_bonus, qty_gift,
    revenue_paid, discount_total, transactions_count,
    had_promo, promo_type, was_in_assortment, lifecycle_state_at_date, was_oos,
    etl_run_id, source_load_id
)
SELECT
    rl.event_time::date                                                 AS as_of_date,
    rl.location_id,
    rl.product_id,
    SUM(CASE WHEN rl.line_kind = 'sale'        THEN rl.qty ELSE 0 END)  AS qty_sold,
    SUM(CASE WHEN rl.line_kind = 'return'      THEN rl.qty ELSE 0 END)  AS qty_returned,
    SUM(CASE WHEN rl.line_kind = 'promo_bonus' THEN rl.qty ELSE 0 END)  AS qty_promo_bonus,
    SUM(CASE WHEN rl.line_kind = 'gift'        THEN rl.qty ELSE 0 END)  AS qty_gift,
    SUM(CASE WHEN rl.line_kind = 'sale'
             THEN rl.qty * rl.unit_price_paid ELSE 0 END)               AS revenue_paid,
    SUM(rl.discount_amount)                                             AS discount_total,
    COUNT(DISTINCT rl.receipt_id)                                       AS transactions_count,
    bool_or(p.promo_id IS NOT NULL)                                     AS had_promo,
    MIN(p.type)                                                         AS promo_type,
    bool_or(sa.product_id IS NOT NULL)                                  AS was_in_assortment,
    MIN(sa.lifecycle_state)                                             AS lifecycle_state_at_date,
    bool_or(COALESCE(soh.qty_on_hand, 0) = 0)                           AS was_oos,
    $1                                                                  AS etl_run_id,
    $2                                                                  AS source_load_id
FROM   pg_temp.stg_receipt_line rl
LEFT JOIN pg_temp.stg_promo p
       ON p.product_id = rl.product_id
      AND rl.event_time::date BETWEEN p.date_from AND p.date_to
LEFT JOIN pg_temp.stg_store_assortment sa
       ON sa.product_id  = rl.product_id
      AND sa.location_id = rl.location_id
      AND rl.event_time::date
          BETWEEN COALESCE(sa.effective_from, sa.valid_from)
              AND COALESCE(sa.effective_to,  sa.valid_to, '9999-12-31'::date)
LEFT JOIN pg_temp.stg_stock_on_hand soh
       ON soh.product_id  = rl.product_id
      AND soh.location_id = rl.location_id
      AND soh.as_of_date  = rl.event_time::date
GROUP  BY rl.event_time::date, rl.location_id, rl.product_id
ON CONFLICT (product_id, location_id, as_of_date) DO UPDATE
   SET qty_sold                = EXCLUDED.qty_sold,
       qty_returned            = EXCLUDED.qty_returned,
       qty_promo_bonus         = EXCLUDED.qty_promo_bonus,
       qty_gift                = EXCLUDED.qty_gift,
       revenue_paid            = EXCLUDED.revenue_paid,
       discount_total          = EXCLUDED.discount_total,
       transactions_count      = EXCLUDED.transactions_count,
       had_promo               = EXCLUDED.had_promo,
       promo_type              = EXCLUDED.promo_type,
       was_in_assortment       = EXCLUDED.was_in_assortment,
       lifecycle_state_at_date = EXCLUDED.lifecycle_state_at_date,
       was_oos                 = EXCLUDED.was_oos,
       etl_run_id              = EXCLUDED.etl_run_id,
       source_load_id          = EXCLUDED.source_load_id,
       created_at              = now();
