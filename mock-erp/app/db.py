"""PostgreSQL connection pool + bootstrap for mock-erp.

Uses psycopg (v3) with connection pooling. The mock-erp database is created
on startup if it does not yet exist (we open an admin connection to the
``postgres`` system database for the CREATE DATABASE step).

Schema is initialized via ``app/migrations/0001_init.sql`` — applied once,
idempotent (CREATE TABLE IF NOT EXISTS).
"""
from __future__ import annotations

import logging
import os
import re
from collections.abc import Generator
from contextlib import contextmanager
from pathlib import Path

import psycopg
from psycopg.rows import dict_row
from psycopg_pool import ConnectionPool

log = logging.getLogger("mock_erp.db")


def _dsn() -> str:
    return os.getenv(
        "MOCK_ERP_DSN",
        "postgres://e_zoo_app:ezoo_app_dev@postgres:5432/mock_erp_db",
    )


def _admin_dsn(target_dsn: str) -> tuple[str, str]:
    """Return (admin_dsn, target_db_name).

    Admin DSN connects to the ``postgres`` system DB on the same host. We use
    ``POSTGRES_USER`` / ``POSTGRES_PASSWORD`` from env (the superuser created by
    the postgres image) so we have privileges to CREATE DATABASE.
    """
    # Parse host/port from the target DSN.
    m = re.match(
        r"postgres(?:ql)?://([^:]+):([^@]+)@([^:/]+)(?::(\d+))?/([^?]+)(?:\?.*)?$",
        target_dsn,
    )
    if not m:
        raise RuntimeError(f"unsupported DSN shape: {target_dsn!r}")
    _user, _pwd, host, port, dbname = m.groups()
    port = port or "5432"
    admin_user = os.getenv("POSTGRES_USER", "adapter")
    admin_pwd = os.getenv("POSTGRES_PASSWORD", "adapter")
    admin = f"postgres://{admin_user}:{admin_pwd}@{host}:{port}/postgres"
    return admin, dbname


def _ensure_database(target_dsn: str) -> None:
    """Create target DB if missing (uses admin connection to ``postgres``)."""
    admin_dsn, db_name = _admin_dsn(target_dsn)
    if not re.match(r"^[A-Za-z0-9_]+$", db_name):
        raise RuntimeError(f"invalid DB name: {db_name!r}")
    log.info("ensuring database %s exists via admin connection", db_name)
    with psycopg.connect(admin_dsn, autocommit=True) as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT 1 FROM pg_database WHERE datname = %s", (db_name,))
            if cur.fetchone() is None:
                # Match the target DSN's user as owner so it can fully manage.
                m = re.match(r"postgres(?:ql)?://([^:]+):", target_dsn)
                owner = m.group(1) if m else "e_zoo_app"
                cur.execute(f'CREATE DATABASE "{db_name}" OWNER "{owner}"')
                log.info("created database %s owner=%s", db_name, owner)
            else:
                log.info("database %s already exists", db_name)


def _migrations_dir() -> Path:
    return Path(__file__).resolve().parent / "migrations"


def _apply_migrations(conn: psycopg.Connection) -> None:
    """Apply all .sql files in app/migrations/ in lexical order. Idempotent."""
    mdir = _migrations_dir()
    files = sorted(mdir.glob("*.sql"))
    if not files:
        log.warning("no migration files found in %s", mdir)
        return
    with conn.cursor() as cur:
        for f in files:
            log.info("applying migration %s", f.name)
            cur.execute(f.read_text(encoding="utf-8"))
    conn.commit()


_pool: ConnectionPool | None = None


def get_pool() -> ConnectionPool:
    if _pool is None:
        raise RuntimeError("connection pool not initialized; call init_db() first")
    return _pool


def init_db() -> None:
    """Idempotent bootstrap: ensure DB exists, apply schema, open pool."""
    global _pool
    target = _dsn()
    _ensure_database(target)
    if _pool is None:
        _pool = ConnectionPool(
            conninfo=target,
            min_size=2,
            max_size=int(os.getenv("MOCK_ERP_POOL_MAX", "20")),
            kwargs={"autocommit": False},
            open=True,
        )
        log.info("psycopg pool open dsn=%s", target)
    with _pool.connection() as conn:
        _apply_migrations(conn)


@contextmanager
def get_conn() -> Generator[psycopg.Connection, None, None]:
    """Context-managed connection (transaction auto-committed on exit)."""
    pool = get_pool()
    with pool.connection() as conn:
        yield conn


def fetch_dict_rows(conn: psycopg.Connection, sql: str, params: tuple = ()) -> list[dict]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, params)
        return list(cur.fetchall())


def fetch_one_dict(conn: psycopg.Connection, sql: str, params: tuple = ()) -> dict | None:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, params)
        return cur.fetchone()
