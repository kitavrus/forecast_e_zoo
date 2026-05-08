"""GET /api/v1/{entity} for all 16 source-adapter entities (psycopg backend).

Master entities: ?since=<ISO-8601>&cursor=<opaque>&limit=10000
Facts entities:  ?date_from=<YYYY-MM-DD>&date_to=<YYYY-MM-DD>&cursor=...&limit=...

Response: JSON array
Headers: X-Total-Count, X-Next-Cursor
"""
from __future__ import annotations

from typing import Any

from fastapi import APIRouter, Depends, Query, Response
from psycopg.rows import dict_row

from app.auth import require_api_key
from app.db import get_pool
from app.models import (
    ENTITY_RESPONSE_COLUMNS,
    FACTS_ENTITIES,
    MASTER_ENTITIES,
    parse_date,
    parse_iso,
    row_to_response,
)
from app.pagination import clamp_limit, decode_cursor, encode_cursor

router = APIRouter(prefix="/api/v1", dependencies=[Depends(require_api_key)])


def _set_headers(response: Response, total: int, next_cursor: str) -> None:
    response.headers["X-Total-Count"] = str(total)
    response.headers["X-Next-Cursor"] = next_cursor


def _is_int_pk(table: str, pk_col: str) -> bool:
    # row_id and "id" of facts tables are bigserial integers.
    if pk_col == "row_id":
        return True
    if pk_col == "id" and table in {"receipt_line", "stock_movement"}:
        return True
    return False


def _master_query(
    table: str, pk_col: str, updated_col: str | None,
    *, since: str | None, cursor: str | None, limit: int,
) -> tuple[list[dict], int, str]:
    where: list[str] = []
    args: list[Any] = []
    if updated_col is not None and since:
        since_dt = parse_iso(since)
        if since_dt is not None:
            where.append(f"{updated_col} >= %s")
            args.append(since_dt)

    last_pk = decode_cursor(cursor)
    if last_pk is not None:
        if _is_int_pk(table, pk_col):
            try:
                where.append(f"{pk_col} > %s")
                args.append(int(last_pk))
            except ValueError:
                pass
        else:
            where.append(f"{pk_col} > %s")
            args.append(last_pk)

    cols = ENTITY_RESPONSE_COLUMNS[table]
    select_cols = ", ".join([*cols, pk_col]) if pk_col not in cols else ", ".join(cols)
    where_sql = (" WHERE " + " AND ".join(where)) if where else ""
    sql = (
        f"SELECT {select_cols} FROM {table}{where_sql} "
        f"ORDER BY {pk_col} LIMIT %s"
    )
    count_sql = f"SELECT COUNT(*) FROM {table}{where_sql}"

    pool = get_pool()
    with pool.connection() as conn:
        with conn.cursor(row_factory=dict_row) as cur:
            cur.execute(sql, (*args, limit))
            rows = list(cur.fetchall())
        with conn.cursor() as cur:
            cur.execute(count_sql, tuple(args))
            total = int(cur.fetchone()[0])

    next_cur = ""
    if len(rows) == limit and rows:
        last_value = rows[-1][pk_col]
        next_cur = encode_cursor(last_value)
    return rows, total, next_cur


def _facts_query(
    table: str, pk_col: str, date_col: str,
    *, date_from: str | None, date_to: str | None, cursor: str | None, limit: int,
) -> tuple[list[dict], int, str]:
    where: list[str] = []
    args: list[Any] = []
    df = parse_date(date_from)
    dt = parse_date(date_to)
    if df is not None:
        where.append(f"{date_col} >= %s")
        args.append(df)
    if dt is not None:
        where.append(f"{date_col} <= %s")
        args.append(dt)

    last_pk = decode_cursor(cursor)
    if last_pk is not None:
        if _is_int_pk(table, pk_col):
            try:
                where.append(f"{pk_col} > %s")
                args.append(int(last_pk))
            except ValueError:
                pass
        else:
            where.append(f"{pk_col} > %s")
            args.append(last_pk)

    cols = ENTITY_RESPONSE_COLUMNS[table]
    select_cols = ", ".join([*cols, pk_col]) if pk_col not in cols else ", ".join(cols)
    where_sql = (" WHERE " + " AND ".join(where)) if where else ""
    sql = (
        f"SELECT {select_cols} FROM {table}{where_sql} "
        f"ORDER BY {pk_col} LIMIT %s"
    )
    count_sql = f"SELECT COUNT(*) FROM {table}{where_sql}"

    pool = get_pool()
    with pool.connection() as conn:
        with conn.cursor(row_factory=dict_row) as cur:
            cur.execute(sql, (*args, limit))
            rows = list(cur.fetchall())
        with conn.cursor() as cur:
            cur.execute(count_sql, tuple(args))
            total = int(cur.fetchone()[0])

    next_cur = ""
    if len(rows) == limit and rows:
        last_value = rows[-1][pk_col]
        next_cur = encode_cursor(last_value)
    return rows, total, next_cur


def _serve_master(
    table: str, response: Response, since: str | None,
    cursor: str | None, limit: int,
) -> list[dict]:
    pk_col, updated_col = MASTER_ENTITIES[table]
    rows, total, nxt = _master_query(
        table, pk_col, updated_col,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    cols = ENTITY_RESPONSE_COLUMNS[table]
    return [row_to_response(r, cols) for r in rows]


def _serve_facts(
    table: str, response: Response, date_from: str | None,
    date_to: str | None, cursor: str | None, limit: int,
) -> list[dict]:
    pk_col, date_col = FACTS_ENTITIES[table]
    rows, total, nxt = _facts_query(
        table, pk_col, date_col,
        date_from=date_from, date_to=date_to, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    cols = ENTITY_RESPONSE_COLUMNS[table]
    return [row_to_response(r, cols) for r in rows]


# --- Master endpoints (12) ---

@router.get("/products")
def get_products(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("products", response, since, cursor, limit)


@router.get("/product_barcodes")
def get_product_barcodes(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("product_barcodes", response, since, cursor, limit)


@router.get("/category")
def get_category(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("category", response, since, cursor, limit)


@router.get("/location")
def get_location(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("location", response, since, cursor, limit)


@router.get("/supplier")
def get_supplier(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("supplier", response, since, cursor, limit)


@router.get("/supply_spec")
def get_supply_spec(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("supply_spec", response, since, cursor, limit)


@router.get("/promo")
def get_promo(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("promo", response, since, cursor, limit)


@router.get("/order_rule")
def get_order_rule(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("order_rule", response, since, cursor, limit)


@router.get("/supply_plan")
def get_supply_plan(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("supply_plan", response, since, cursor, limit)


@router.get("/master_change_log")
def get_master_change_log(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("master_change_log", response, since, cursor, limit)


@router.get("/store_assortment")
def get_store_assortment(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("store_assortment", response, since, cursor, limit)


@router.get("/store_assortment_lifecycle_events")
def get_store_assortment_lifecycle_events(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_master("store_assortment_lifecycle_events", response, since, cursor, limit)


# --- Facts endpoints (4) ---

@router.get("/receipt_line")
def get_receipt_line(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_facts("receipt_line", response, date_from, date_to, cursor, limit)


@router.get("/location_stock_snapshot")
def get_location_stock_snapshot(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_facts("location_stock_snapshot", response, date_from, date_to, cursor, limit)


@router.get("/stock_movement")
def get_stock_movement(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_facts("stock_movement", response, date_from, date_to, cursor, limit)


@router.get("/supplier_stock_snapshot")
def get_supplier_stock_snapshot(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
) -> list[dict]:
    return _serve_facts("supplier_stock_snapshot", response, date_from, date_to, cursor, limit)
