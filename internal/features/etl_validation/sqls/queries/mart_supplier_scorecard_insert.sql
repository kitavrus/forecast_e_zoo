-- Агрегация по supplier + неделя из stg_receiving_details
-- → marts.mart_supplier_scorecard с upsert по PK (supplier_id, week_start).
-- Параметры: $1 etl_run_id (uuid), $2 source_load_id (uuid).
INSERT INTO marts.mart_supplier_scorecard (
    supplier_id, week_start,
    fill_rate_avg, otif_pct, lead_time_actual_avg,
    qty_short_total, qty_damaged_total, qty_returned_total,
    lines_delivered, lines_late,
    etl_run_id, source_load_id
)
SELECT supplier_id,
       date_trunc('week', delivery_date)::date          AS week_start,
       AVG(fill_rate)                                   AS fill_rate_avg,
       AVG(CASE WHEN otif THEN 1.0 ELSE 0.0 END) * 100  AS otif_pct,
       AVG(lead_time_actual)                            AS lead_time_actual_avg,
       SUM(qty_short)                                   AS qty_short_total,
       SUM(qty_damaged)                                 AS qty_damaged_total,
       SUM(qty_returned)                                AS qty_returned_total,
       COUNT(*)::int                                    AS lines_delivered,
       SUM(CASE WHEN late THEN 1 ELSE 0 END)::int       AS lines_late,
       $1, $2
FROM   pg_temp.stg_receiving_details
GROUP  BY supplier_id, date_trunc('week', delivery_date)
ON CONFLICT (supplier_id, week_start) DO UPDATE
   SET fill_rate_avg        = EXCLUDED.fill_rate_avg,
       otif_pct              = EXCLUDED.otif_pct,
       lead_time_actual_avg  = EXCLUDED.lead_time_actual_avg,
       qty_short_total       = EXCLUDED.qty_short_total,
       qty_damaged_total     = EXCLUDED.qty_damaged_total,
       qty_returned_total    = EXCLUDED.qty_returned_total,
       lines_delivered       = EXCLUDED.lines_delivered,
       lines_late            = EXCLUDED.lines_late,
       etl_run_id            = EXCLUDED.etl_run_id,
       source_load_id        = EXCLUDED.source_load_id,
       created_at            = now();
