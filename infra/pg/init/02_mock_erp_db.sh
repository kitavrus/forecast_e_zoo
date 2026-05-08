#!/bin/bash
# Init script: создаёт логическую БД mock_erp_db (для mock-erp сервиса) внутри
# того же контейнера ezoo_pg. Mock-erp подключается с DSN
#   postgres://e_zoo_app:...@postgres:5432/mock_erp_db
# и владеет своей БД целиком (table-management, миграции схемы — на стороне
# mock-erp, через app/migrations/0001_init.sql).
#
# Этот скрипт работает только при первой инициализации тома (когда volume
# pg_data пуст). На существующем volume mock-erp создаст БД сам через
# admin-connection (см. mock-erp/app/db.py:_ensure_database).
#
# Выполняется ПОСЛЕ 01_init.sh — поэтому role e_zoo_app уже существует.

set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Idempotent: создаём БД только если её ещё нет.
    SELECT 'CREATE DATABASE mock_erp_db OWNER e_zoo_app'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'mock_erp_db')
    \gexec

    GRANT ALL PRIVILEGES ON DATABASE mock_erp_db TO e_zoo_app;
EOSQL

echo "✓ infra/pg/init/02_mock_erp_db.sh: mock_erp_db created (owner=e_zoo_app)"
