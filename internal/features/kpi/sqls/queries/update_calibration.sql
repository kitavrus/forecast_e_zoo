-- PUT /v1/kpi/calibrations/:id — обновить params (jsonb).
-- $1 = id (UUID), $2 = params (JSONB).
UPDATE kpi.kpi_calibrations
   SET params     = $2::jsonb,
       updated_at = now()
 WHERE id = $1::uuid
RETURNING id, kpi_name, scope_type, scope_id, params, created_at, updated_at
