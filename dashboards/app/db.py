"""PostgreSQL connection pool (read-only queries) using psycopg3."""
from __future__ import annotations

import logging
from contextlib import contextmanager
from typing import Any, Iterator

from psycopg import Connection
from psycopg.rows import dict_row
from psycopg_pool import ConnectionPool

logger = logging.getLogger(__name__)

_pool: ConnectionPool | None = None


def init_pool(dsn: str) -> None:
    """Create the global connection pool. Tolerates DB being temporarily unavailable."""
    global _pool
    if _pool is not None:
        return
    _pool = ConnectionPool(
        conninfo=dsn,
        min_size=1,
        max_size=5,
        timeout=10,
        max_idle=120,
        open=False,
    )
    try:
        _pool.open(wait=False)
    except Exception as exc:  # noqa: BLE001 — graceful, retry on next query
        logger.warning("connection pool open failed: %s", exc)


def close_pool() -> None:
    global _pool
    if _pool is not None:
        try:
            _pool.close()
        except Exception:  # noqa: BLE001
            pass
        _pool = None


@contextmanager
def get_conn() -> Iterator[Connection]:
    if _pool is None:
        raise RuntimeError("connection pool not initialised")
    with _pool.connection() as conn:
        yield conn


def fetch_all(sql: str, params: tuple[Any, ...] | None = None) -> list[dict[str, Any]]:
    """Run SELECT, return list of dict rows. Returns [] on any error."""
    try:
        with get_conn() as conn:
            with conn.cursor(row_factory=dict_row) as cur:
                cur.execute(sql, params or ())
                rows = cur.fetchall()
                return list(rows)
    except Exception as exc:  # noqa: BLE001 — graceful empty result on missing tables
        logger.warning("query failed (%s): %s", sql.split()[0:3], exc)
        return []


def fetch_one(sql: str, params: tuple[Any, ...] | None = None) -> dict[str, Any] | None:
    rows = fetch_all(sql, params)
    return rows[0] if rows else None


def fetch_scalar(sql: str, params: tuple[Any, ...] | None = None, default: Any = 0) -> Any:
    """Return first column of first row; default if missing."""
    row = fetch_one(sql, params)
    if not row:
        return default
    return next(iter(row.values()), default)
