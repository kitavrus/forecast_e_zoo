"""Admin endpoints for the on-demand day-by-day seeder.

All endpoints require the standard mock-erp X-API-Key auth.

| Method | Path                       | Purpose                                         |
|--------|----------------------------|-------------------------------------------------|
| GET    | /admin/seed/state          | Return current seed state                        |
| POST   | /admin/seed/initial        | Run master bootstrap (idempotent)                |
| POST   | /admin/seed/days?count=N   | Generate N days of facts; advance current_date  |
| POST   | /admin/seed/next-day       | Alias for ?count=1                               |
| POST   | /admin/seed/reset?confirm=true | Hard wipe all data + reset state             |
"""
from __future__ import annotations

import logging
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Query, status

from app.auth import require_api_key
from app.db import get_pool
from app.seeder import get_state, reset_all, seed_days, seed_master

log = logging.getLogger("mock_erp.routes.seed")

router = APIRouter(prefix="/admin/seed", dependencies=[Depends(require_api_key)])


@router.get("/state")
def state() -> dict[str, Any]:
    pool = get_pool()
    with pool.connection() as conn:
        return get_state(conn)


@router.post("/initial", status_code=status.HTTP_201_CREATED)
def initial() -> dict[str, Any]:
    pool = get_pool()
    with pool.connection() as conn:
        return seed_master(conn)


@router.post("/days")
def days(count: int = Query(default=1, ge=1, le=365)) -> dict[str, Any]:
    pool = get_pool()
    try:
        with pool.connection() as conn:
            return seed_days(conn, count)
    except RuntimeError as exc:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc


@router.post("/next-day")
def next_day() -> dict[str, Any]:
    pool = get_pool()
    try:
        with pool.connection() as conn:
            return seed_days(conn, 1)
    except RuntimeError as exc:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail=str(exc)) from exc


@router.post("/reset")
def reset(confirm: bool = Query(default=False)) -> dict[str, Any]:
    if not confirm:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="reset requires ?confirm=true",
        )
    pool = get_pool()
    with pool.connection() as conn:
        return reset_all(conn)
