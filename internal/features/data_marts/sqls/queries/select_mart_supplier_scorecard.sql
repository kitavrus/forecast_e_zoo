-- select_mart_supplier_scorecard.sql
-- Cursor pagination по PK (supplier_id, week_start).
-- Параметры:
--   $1 = etl_run_id (uuid)
--   $2 = last_supplier_id (text)
--   $3 = last_week_start  (date)
--   $4 = limit (int)
SELECT
    supplier_id,
    week_start,
    fill_rate_avg,
    otif_pct,
    lead_time_actual_avg,
    qty_short_total,
    qty_damaged_total,
    qty_returned_total,
    lines_delivered,
    lines_late,
    etl_run_id,
    source_load_id,
    created_at
FROM marts.mart_supplier_scorecard
WHERE etl_run_id = $1
  AND (supplier_id, week_start) > ($2, $3)
ORDER BY supplier_id, week_start
LIMIT $4;
