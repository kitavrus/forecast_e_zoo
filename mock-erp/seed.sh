#!/bin/bash
# mock-erp entrypoint.
#
# Schema is created/applied automatically on FastAPI startup (lifespan handler
# calls app.db.init_db()), which:
#   1) creates the mock_erp_db database in the shared `ezoo_pg` instance
#      via an admin connection (uses POSTGRES_USER/POSTGRES_PASSWORD env);
#   2) applies app/migrations/*.sql idempotently;
#   3) opens the psycopg connection pool.
#
# Master seed is NOT run automatically. Trigger it explicitly via:
#   POST /admin/seed/initial   (master entities only, ~1-3s at default scale)
#   POST /admin/seed/days?count=N   (generate N days of facts)
#
# This is by design - a fresh container starts with an empty fact stream and
# advances day-by-day, so you can drive demos, dashboards and E2E tests at
# whatever pace you choose.
set -e
exec uvicorn app.main:app --host 0.0.0.0 --port 8090
