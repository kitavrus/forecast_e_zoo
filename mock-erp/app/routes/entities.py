"""GET /api/v1/{entity} for all 16 source-adapter entities.

Master entities: ?since=<ISO-8601>&cursor=<opaque>&limit=10000
Facts entities:  ?date_from=<YYYY-MM-DD>&date_to=<YYYY-MM-DD>&cursor=...&limit=...

Response: JSON array
Headers: X-Total-Count, X-Next-Cursor
"""
from __future__ import annotations

from typing import Any

from fastapi import APIRouter, Depends, Query, Response
from sqlalchemy import func
from sqlmodel import Session, select

from app.auth import require_api_key
from app.db import get_session
from app.models import (
    Category,
    Location,
    LocationStockSnapshot,
    MasterChangeLog,
    OrderRule,
    Product,
    ProductBarcode,
    Promo,
    ReceiptLine,
    StockMovement,
    StoreAssortment,
    StoreAssortmentLifecycleEvent,
    Supplier,
    SupplierStockSnapshot,
    SupplyPlan,
    SupplySpec,
    model_to_dict,
    parse_date,
    parse_iso,
)
from app.pagination import clamp_limit, decode_cursor, encode_cursor

router = APIRouter(prefix="/api/v1", dependencies=[Depends(require_api_key)])


def _master_query(
    session: Session,
    model: type,
    pk_col,
    updated_col,
    *,
    since: str | None,
    cursor: str | None,
    limit: int,
) -> tuple[list[Any], int, str]:
    """Generic master entity query with since + cursor."""
    base = select(model)
    count_q = select(func.count()).select_from(model)
    since_dt = parse_iso(since) if since else None
    if since_dt is not None and updated_col is not None:
        base = base.where(updated_col >= since_dt)
        count_q = count_q.where(updated_col >= since_dt)

    last_pk = decode_cursor(cursor)
    if last_pk is not None:
        base = base.where(pk_col > last_pk)

    base = base.order_by(pk_col).limit(limit)
    rows = session.exec(base).all()
    total = session.exec(count_q).one()

    next_cur = ""
    if len(rows) == limit and rows:
        last_value = getattr(rows[-1], pk_col.key)
        next_cur = encode_cursor(last_value)
    return rows, int(total), next_cur


def _facts_query(
    session: Session,
    model: type,
    pk_col,
    date_col,
    *,
    date_from: str | None,
    date_to: str | None,
    cursor: str | None,
    limit: int,
) -> tuple[list[Any], int, str]:
    """Generic facts entity query with date range + cursor on PK."""
    base = select(model)
    count_q = select(func.count()).select_from(model)
    df = parse_date(date_from)
    dt = parse_date(date_to)
    if df is not None:
        base = base.where(date_col >= df)
        count_q = count_q.where(date_col >= df)
    if dt is not None:
        base = base.where(date_col <= dt)
        count_q = count_q.where(date_col <= dt)

    last_pk = decode_cursor(cursor)
    if last_pk is not None:
        try:
            last_pk_typed: Any = int(last_pk)
        except ValueError:
            last_pk_typed = last_pk
        base = base.where(pk_col > last_pk_typed)

    base = base.order_by(pk_col).limit(limit)
    rows = session.exec(base).all()
    total = session.exec(count_q).one()

    next_cur = ""
    if len(rows) == limit and rows:
        last_value = getattr(rows[-1], pk_col.key)
        next_cur = encode_cursor(last_value)
    return rows, int(total), next_cur


def _set_headers(response: Response, total: int, next_cursor: str) -> None:
    response.headers["X-Total-Count"] = str(total)
    response.headers["X-Next-Cursor"] = next_cursor


# --- Master endpoints (12) ---


@router.get("/products")
def get_products(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, Product, Product.id, Product.updated_at,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/product_barcodes")
def get_product_barcodes(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, ProductBarcode, ProductBarcode.barcode, None,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/category")
def get_category(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, Category, Category.id, Category.updated_at,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/location")
def get_location(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, Location, Location.id, Location.updated_at,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/supplier")
def get_supplier(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, Supplier, Supplier.id, Supplier.updated_at,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/supply_spec")
def get_supply_spec(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, SupplySpec, SupplySpec.row_id, SupplySpec.valid_from,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/promo")
def get_promo(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, Promo, Promo.id, Promo.updated_at,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/order_rule")
def get_order_rule(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, OrderRule, OrderRule.id, OrderRule.valid_from,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/supply_plan")
def get_supply_plan(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, SupplyPlan, SupplyPlan.id, SupplyPlan.plan_date,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/master_change_log")
def get_master_change_log(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, MasterChangeLog, MasterChangeLog.row_id, MasterChangeLog.changed_at,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/store_assortment")
def get_store_assortment(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, StoreAssortment, StoreAssortment.row_id, StoreAssortment.updated_at,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/store_assortment_lifecycle_events")
def get_store_assortment_lifecycle_events(
    response: Response,
    since: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _master_query(
        session, StoreAssortmentLifecycleEvent,
        StoreAssortmentLifecycleEvent.row_id,
        StoreAssortmentLifecycleEvent.event_date,
        since=since, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


# --- Facts endpoints (4) ---


@router.get("/receipt_line")
def get_receipt_line(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _facts_query(
        session, ReceiptLine, ReceiptLine.id, ReceiptLine.event_date,
        date_from=date_from, date_to=date_to, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/location_stock_snapshot")
def get_location_stock_snapshot(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _facts_query(
        session, LocationStockSnapshot, LocationStockSnapshot.row_id,
        LocationStockSnapshot.event_date,
        date_from=date_from, date_to=date_to, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/stock_movement")
def get_stock_movement(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _facts_query(
        session, StockMovement, StockMovement.id, StockMovement.event_date,
        date_from=date_from, date_to=date_to, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]


@router.get("/supplier_stock_snapshot")
def get_supplier_stock_snapshot(
    response: Response,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    cursor: str | None = Query(default=None),
    limit: int = Query(default=10000),
    session: Session = Depends(get_session),
) -> list[dict]:
    rows, total, nxt = _facts_query(
        session, SupplierStockSnapshot, SupplierStockSnapshot.row_id,
        SupplierStockSnapshot.event_date,
        date_from=date_from, date_to=date_to, cursor=cursor, limit=clamp_limit(limit),
    )
    _set_headers(response, total, nxt)
    return [model_to_dict(r) for r in rows]
