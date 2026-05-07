#!/bin/bash
# Init script для PostgreSQL 18 (e_zoo / source_adapter).
# Выполняется один раз при первом старте контейнера postgres
# (когда volume пустой).
#
# Создаёт:
#   * service-роль e_zoo_app (login) с паролем из EZOO_APP_PASSWORD env (или дефолт ezoo_app_dev)
#   * shared NOLOGIN роль mart_reader (для GRANT'ов на marts.*)
#   * базовые grants на public schema
#
# Объекты остальных schemas (marts, kpi, forecast, orders, channels)
# создаются миграциями golang-migrate.

set -e

EZOO_APP_PASSWORD="${EZOO_APP_PASSWORD:-ezoo_app_dev}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Service role для всех 7 микросервисов (PoC: одна общая роль).
    DO \$\$
    BEGIN
       IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'e_zoo_app') THEN
          CREATE ROLE e_zoo_app LOGIN PASSWORD '${EZOO_APP_PASSWORD}';
       END IF;
    END\$\$;

    -- NOLOGIN роль для read-only доступа к marts.*.
    DO \$\$
    BEGIN
       IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'mart_reader') THEN
          CREATE ROLE mart_reader NOLOGIN;
       END IF;
    END\$\$;

    -- Базовые гранты на основную БД.
    GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO e_zoo_app;
    GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO mart_reader;

    -- Гранты на public schema (объекты создаются миграциями).
    GRANT USAGE  ON SCHEMA public TO e_zoo_app;
    GRANT CREATE ON SCHEMA public TO e_zoo_app;
    GRANT USAGE  ON SCHEMA public TO mart_reader;

    -- Default privileges для будущих объектов в public.
    -- TRUNCATE нужен ETL mart-pipeline'у (TRUNCATE+INSERT в transaction для
    -- mart_master_current / mart_calculation_input).
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT
        SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON TABLES   TO e_zoo_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT
        USAGE, SELECT, UPDATE                        ON SEQUENCES TO e_zoo_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT
        EXECUTE                                      ON FUNCTIONS TO e_zoo_app;

    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES    TO mart_reader;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO mart_reader;

    -- Default privileges для будущих объектов, которые сервис-владелец
    -- (роль ${POSTGRES_USER}) создаёт в произвольных схемах.
    -- Применяется к схемам marts/kpi/forecast/orders/channels, которые
    -- создаются миграциями golang-migrate уже после init-скрипта.
    -- USAGE на сами схемы выдаётся в каждой миграции CREATE SCHEMA.
    -- TRUNCATE нужен ETL mart-pipeline'у.
    ALTER DEFAULT PRIVILEGES FOR ROLE ${POSTGRES_USER} GRANT
        SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON TABLES   TO e_zoo_app;
    ALTER DEFAULT PRIVILEGES FOR ROLE ${POSTGRES_USER} GRANT
        USAGE, SELECT, UPDATE          ON SEQUENCES TO e_zoo_app;
    ALTER DEFAULT PRIVILEGES FOR ROLE ${POSTGRES_USER} GRANT
        EXECUTE                        ON FUNCTIONS TO e_zoo_app;
EOSQL

echo "✓ infra/pg/init/01_init.sh: e_zoo_app + mart_reader created"
