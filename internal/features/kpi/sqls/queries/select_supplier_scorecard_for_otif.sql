-- Чтение marts.mart_supplier_scorecard для OTIF.
-- $1 = week_from (DATE), $2 = week_to (DATE).
SELECT
    supplier_id,
    week_start,
    COALESCE(lines_delivered, 0)::int AS lines_delivered,
    COALESCE(lines_late, 0)::int      AS lines_late,
    COALESCE(qty_short_total, 0)::float8 AS qty_short_total,
    fill_rate_avg::float8 AS fill_rate_avg
FROM marts.mart_supplier_scorecard
WHERE week_start BETWEEN $1::date AND $2::date
