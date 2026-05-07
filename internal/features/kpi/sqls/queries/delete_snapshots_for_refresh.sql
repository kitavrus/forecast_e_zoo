-- Удалить снапшоты за дату для пересчёта (POST /v1/kpi/snapshots/refresh).
-- $1 = as_of_date (DATE), $2 = kpi_names ([]TEXT — array; NULL = все).
DELETE FROM kpi.kpi_snapshots
 WHERE as_of_date = $1::date
   AND ($2::text[] IS NULL OR kpi_name = ANY($2::text[]))
