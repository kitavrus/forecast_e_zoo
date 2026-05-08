-- Rollback 1003: revoke CREATE и drop pre-created исторических партиций
-- (оставляем только базовые 2026_05/2026_06, созданные в 1001).
-- Идемпотентно: DROP IF EXISTS.

REVOKE CREATE ON SCHEMA marts FROM e_zoo_app;

DO $$
DECLARE
    base_date DATE;
    part_start DATE;
    part_name  TEXT;
BEGIN
    base_date := date_trunc('month', (now() AT TIME ZONE 'UTC')::date)::date;
    FOR i IN -12..1 LOOP
        part_start := base_date + make_interval(months => i);
        IF part_start IN (DATE '2026-05-01', DATE '2026-06-01') THEN
            CONTINUE;
        END IF;
        part_name := format(
            'mart_demand_history_%s_%s',
            to_char(part_start, 'YYYY'),
            to_char(part_start, 'MM')
        );
        EXECUTE format('DROP TABLE IF EXISTS marts.%I', part_name);

        part_name := format(
            'mart_kpi_daily_%s_%s',
            to_char(part_start, 'YYYY'),
            to_char(part_start, 'MM')
        );
        EXECUTE format('DROP TABLE IF EXISTS marts.%I', part_name);
    END LOOP;
END$$;
