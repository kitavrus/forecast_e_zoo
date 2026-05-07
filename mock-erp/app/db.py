"""SQLite engine + session factory for mock-erp."""
from __future__ import annotations

import os
from collections.abc import Generator

from sqlmodel import Session, SQLModel, create_engine

DB_PATH = os.getenv("DB_PATH", "/data/mock_erp.db")
DB_URL = f"sqlite:///{DB_PATH}"

# check_same_thread=False — sqlmodel/uvicorn workers share the connection;
# WAL journal_mode helps with concurrent reads while a single writer seeds.
_engine = create_engine(
    DB_URL,
    echo=False,
    connect_args={"check_same_thread": False},
)


def get_engine():
    return _engine


def init_db() -> None:
    """Create all tables. Idempotent."""
    SQLModel.metadata.create_all(_engine)
    # WAL + sync NORMAL is enough for the scale we generate.
    with _engine.connect() as conn:
        from sqlalchemy import text

        conn.exec_driver_sql("PRAGMA journal_mode=WAL")
        conn.exec_driver_sql("PRAGMA synchronous=NORMAL")
        conn.exec_driver_sql("PRAGMA temp_store=MEMORY")
        conn.exec_driver_sql("PRAGMA cache_size=-64000")
        conn.commit()
        _ = text  # silence linter


def get_session() -> Generator[Session, None, None]:
    with Session(_engine) as session:
        yield session
