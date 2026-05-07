-- Init script для PostgreSQL 18 (e_zoo / source_adapter).
-- Выполняется один раз при первом старте контейнера postgres
-- (когда volume пустой). После создания БД и POSTGRES_USER —
-- этот скрипт добавляет:
--   * service-роль e_zoo_app (login)
--   * shared NOLOGIN роль mart_reader (для GRANT'ов на marts.*)
--   * базовые grants и search_path для service-роли
--
-- Пароль service-роли берётся из env-переменной EZOO_APP_PASSWORD,
-- которая прокидывается в контейнер через docker-compose (см. .env).
-- Если переменная пуста — используем дефолт (только для dev!).

\set ezoo_app_password `echo "${EZOO_APP_PASSWORD:-ezoo_app_dev}"`

-- Service role для всех 7 микросервисов (PoC: одна общая роль;
-- в проде планируется разделение по сервисам).
DO $$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'e_zoo_app') THEN
      CREATE ROLE e_zoo_app LOGIN PASSWORD :'ezoo_app_password';
   END IF;
END$$;

-- NOLOGIN роль для read-only доступа к marts.*.
DO $$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'mart_reader') THEN
      CREATE ROLE mart_reader NOLOGIN;
   END IF;
END$$;

-- Базовые гранты на основную БД source_adapter:
GRANT CONNECT ON DATABASE source_adapter TO e_zoo_app;
GRANT CONNECT ON DATABASE source_adapter TO mart_reader;

-- Гранты на public schema (objects создаются миграциями).
GRANT USAGE  ON SCHEMA public TO e_zoo_app;
GRANT CREATE ON SCHEMA public TO e_zoo_app;
GRANT USAGE  ON SCHEMA public TO mart_reader;

-- Default privileges для будущих объектов в public:
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT
    SELECT, INSERT, UPDATE, DELETE ON TABLES    TO e_zoo_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT
    USAGE, SELECT, UPDATE                        ON SEQUENCES TO e_zoo_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT
    EXECUTE                                      ON FUNCTIONS TO e_zoo_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES    TO mart_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO mart_reader;
