#!/bin/bash
set -e
DB_PATH=${DB_PATH:-/data/mock_erp.db}
mkdir -p "$(dirname "$DB_PATH")"
if [ ! -s "$DB_PATH" ]; then
  echo "Seeding mock ERP database at $DB_PATH ..."
  python -m app.seeder
fi
exec uvicorn app.main:app --host 0.0.0.0 --port 8090
