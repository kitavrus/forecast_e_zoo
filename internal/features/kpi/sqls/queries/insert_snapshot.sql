-- INSERT новой строки kpi_snapshots.
-- Параметры:
--   $1 = as_of_date (DATE)
--   $2 = kpi_name (TEXT)
--   $3 = scope_type (TEXT)
--   $4 = scope_id (TEXT, nullable)
--   $5 = value (NUMERIC)
--   $6 = calibration_id (UUID, nullable)
--   $7 = etl_run_id (UUID, nullable)
INSERT INTO kpi.kpi_snapshots
    (as_of_date, kpi_name, scope_type, scope_id, value, calibration_id, etl_run_id)
VALUES ($1::date, $2, $3, $4, $5, $6, $7)
RETURNING id, as_of_date, computed_at, created_at
