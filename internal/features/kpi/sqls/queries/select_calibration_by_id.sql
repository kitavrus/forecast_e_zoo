SELECT id, kpi_name, scope_type, scope_id, params, created_at, updated_at
FROM kpi.kpi_calibrations
WHERE id = $1::uuid
LIMIT 1
