-- GET /v1/kpi/snapshots/:id — поиск по UUID.
-- В партиционированной таблице нужен hint на as_of_date для эффективности,
-- но MVP допускает full-partition scan (объём данных ≤ 25K rows/мес).
SELECT
    id,
    as_of_date,
    kpi_name,
    scope_type,
    scope_id,
    value::float8 AS value,
    calibration_id,
    computed_at,
    etl_run_id
FROM kpi.kpi_snapshots
WHERE id = $1::uuid
LIMIT 1
