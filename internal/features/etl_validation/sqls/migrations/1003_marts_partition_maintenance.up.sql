-- Гранты + pre-created исторические партиции для marts.mart_demand_history
-- и marts.mart_kpi_daily (как минимум 12 месяцев назад + 2 вперёд).
--
-- Контекст: ETL pull window = 365 дней; mock-erp seed_days может класть
-- факты в прошлое на SEED_DAYS дней. Без этих партиций upsert падает
-- с "no partition of relation found for row" (SQLSTATE 23514).
--
-- Privileges: e_zoo_app не имеет CREATE на schema marts (миграция 1001
-- даёт только USAGE). Чтобы partitionMaintainer.EnsureNextMonth работал
-- из etl-сервиса (под e_zoo_app), здесь выдаём CREATE; это позволяет
-- DDL'ям из Go-кода создавать новые партиции по мере появления данных.

GRANT CREATE ON SCHEMA marts TO e_zoo_app;

-- mart_demand_history: pre-create 12 месяцев истории + текущий + следующий.
-- Используем DO-блок с динамическим SQL — generate_series не сработает
-- внутри CREATE TABLE PARTITION OF (это DDL).
DO $$
DECLARE
    base_date DATE;
    part_start DATE;
    part_end   DATE;
    part_name  TEXT;
BEGIN
    -- Текущий месяц по UTC (truncated). 14 итераций: -12..+1.
    base_date := date_trunc('month', (now() AT TIME ZONE 'UTC')::date)::date;
    FOR i IN -12..1 LOOP
        part_start := base_date + make_interval(months => i);
        part_end   := part_start + INTERVAL '1 month';
        part_name  := format(
            'mart_demand_history_%s_%s',
            to_char(part_start, 'YYYY'),
            to_char(part_start, 'MM')
        );
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS marts.%I PARTITION OF marts.mart_demand_history '
            'FOR VALUES FROM (%L) TO (%L)',
            part_name, part_start, part_end
        );

        part_name := format(
            'mart_kpi_daily_%s_%s',
            to_char(part_start, 'YYYY'),
            to_char(part_start, 'MM')
        );
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS marts.%I PARTITION OF marts.mart_kpi_daily '
            'FOR VALUES FROM (%L) TO (%L)',
            part_name, part_start, part_end
        );
    END LOOP;
END$$;
