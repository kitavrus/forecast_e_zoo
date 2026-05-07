-- SELECT все калибровки + опциональные фильтры.
-- $1 = kpi_name (TEXT, nullable), $2 = scope_type (TEXT, nullable).
SELECT
    id,
    kpi_name,
    scope_type,
    scope_id,
    params,
    created_at,
    updated_at
FROM kpi.kpi_calibrations
WHERE ($1::text IS NULL OR kpi_name   = $1::text)
  AND ($2::text IS NULL OR scope_type = $2::text)
ORDER BY kpi_name, scope_type, scope_id NULLS FIRST
