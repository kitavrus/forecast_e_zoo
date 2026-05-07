"""SQLModel definitions for all 16 mock-erp entities + received_orders.

Field shape mirrors the testdata/fixtures/*.json contract used by
internal/features/data_export/loader/erp_e_zoo_reader.go.

JSON columns are stored as TEXT and (de)serialized in route handlers via
helpers in this module.
"""
from __future__ import annotations

import json
from datetime import datetime
from typing import Any

from sqlalchemy import Column, Index
from sqlalchemy import JSON as SAJSON
from sqlmodel import Field, SQLModel


def _json_col() -> Column:
    return Column(SAJSON, nullable=True)


# --- Master entities ---


class Product(SQLModel, table=True):
    __tablename__ = "products"
    id: str = Field(primary_key=True)
    sku: str = Field(index=True)
    name: str
    category_id: str = Field(index=True)
    unit: str
    pack_size: float
    is_active: bool = True
    attributes: dict[str, Any] | None = Field(default=None, sa_column=_json_col())
    updated_at: datetime = Field(index=True)


class ProductBarcode(SQLModel, table=True):
    __tablename__ = "product_barcodes"
    barcode: str = Field(primary_key=True)
    product_id: str = Field(index=True)


class Category(SQLModel, table=True):
    __tablename__ = "category"
    id: str = Field(primary_key=True)
    name: str
    path: str
    updated_at: datetime = Field(index=True)


class Location(SQLModel, table=True):
    __tablename__ = "location"
    id: str = Field(primary_key=True)
    type: str
    name: str
    region: str
    updated_at: datetime = Field(index=True)


class Supplier(SQLModel, table=True):
    __tablename__ = "supplier"
    id: str = Field(primary_key=True)
    name: str
    inn: str | None = None
    updated_at: datetime = Field(index=True)


class SupplySpec(SQLModel, table=True):
    __tablename__ = "supply_spec"
    # Composite PK (product_id, supplier_id, valid_from); use surrogate row id.
    row_id: int | None = Field(default=None, primary_key=True)
    product_id: str = Field(index=True)
    supplier_id: str = Field(index=True)
    pack_qty: int
    lead_time_days: int
    min_order_qty: int
    multiple: int
    valid_from: datetime = Field(index=True)


class Promo(SQLModel, table=True):
    __tablename__ = "promo"
    id: str = Field(primary_key=True)
    location_id: str = Field(index=True)
    product_id: str = Field(index=True)
    start_date: datetime
    end_date: datetime
    discount_pct: int
    updated_at: datetime = Field(index=True)


class OrderRule(SQLModel, table=True):
    __tablename__ = "order_rule"
    id: str = Field(primary_key=True)
    location_id: str = Field(index=True)
    rule_type: str
    payload: dict[str, Any] | None = Field(default=None, sa_column=_json_col())
    valid_from: datetime = Field(index=True)


class SupplyPlan(SQLModel, table=True):
    __tablename__ = "supply_plan"
    id: str = Field(primary_key=True)
    location_id: str = Field(index=True)
    product_id: str = Field(index=True)
    supplier_id: str = Field(index=True)
    plan_date: datetime = Field(index=True)
    qty: int


class StoreAssortment(SQLModel, table=True):
    __tablename__ = "store_assortment"
    row_id: int | None = Field(default=None, primary_key=True)
    location_id: str = Field(index=True)
    product_id: str = Field(index=True)
    start_date: datetime
    is_active: bool = True
    updated_at: datetime = Field(index=True)


class StoreAssortmentLifecycleEvent(SQLModel, table=True):
    __tablename__ = "store_assortment_lifecycle_events"
    row_id: int | None = Field(default=None, primary_key=True)
    location_id: str = Field(index=True)
    product_id: str = Field(index=True)
    event_type: str
    event_date: datetime = Field(index=True)
    payload: dict[str, Any] | None = Field(default=None, sa_column=_json_col())


class MasterChangeLog(SQLModel, table=True):
    __tablename__ = "master_change_log"
    row_id: int | None = Field(default=None, primary_key=True)
    entity: str = Field(index=True)
    entity_pk: dict[str, Any] | None = Field(default=None, sa_column=_json_col())
    field: str
    old_value: str | None = None
    new_value: str | None = None
    changed_at: datetime = Field(index=True)


# --- Facts entities ---


class ReceiptLine(SQLModel, table=True):
    __tablename__ = "receipt_line"
    id: int | None = Field(default=None, primary_key=True)
    receipt_id: str = Field(index=True)
    location_id: str = Field(index=True)
    product_id: str = Field(index=True)
    qty: int
    price: float
    event_time: datetime
    event_date: datetime = Field(index=True)
    payload: dict[str, Any] | None = Field(default=None, sa_column=_json_col())


class LocationStockSnapshot(SQLModel, table=True):
    __tablename__ = "location_stock_snapshot"
    row_id: int | None = Field(default=None, primary_key=True)
    event_date: datetime = Field(index=True)
    location_id: str = Field(index=True)
    product_id: str = Field(index=True)
    qty_on_hand: int
    qty_reserved: int
    as_of: datetime


class StockMovement(SQLModel, table=True):
    __tablename__ = "stock_movement"
    id: int | None = Field(default=None, primary_key=True)
    event_date: datetime = Field(index=True)
    event_time: datetime
    location_id: str = Field(index=True)
    product_id: str = Field(index=True)
    movement_type: str
    qty: int
    ref_id: str
    payload: dict[str, Any] | None = Field(default=None, sa_column=_json_col())


class SupplierStockSnapshot(SQLModel, table=True):
    __tablename__ = "supplier_stock_snapshot"
    row_id: int | None = Field(default=None, primary_key=True)
    event_date: datetime = Field(index=True)
    supplier_id: str = Field(index=True)
    product_id: str = Field(index=True)
    qty: int
    as_of: datetime


# --- Received orders (write side) ---


class ReceivedOrder(SQLModel, table=True):
    __tablename__ = "received_orders"
    id: int | None = Field(default=None, primary_key=True)
    external_ref: str = Field(index=True)
    po_number: str | None = Field(default=None, index=True)
    supplier_id: str | None = Field(default=None, index=True)
    location_id: str | None = Field(default=None, index=True)
    accepted_at: datetime = Field(index=True)
    raw_body: str  # JSON-encoded full request


# Composite indexes for facts entity range scans.
Index(
    "ix_receipt_line_event_date_id",
    ReceiptLine.event_date,
    ReceiptLine.id,
)
Index(
    "ix_location_stock_event_date_row",
    LocationStockSnapshot.event_date,
    LocationStockSnapshot.row_id,
)
Index(
    "ix_stock_movement_event_date_id",
    StockMovement.event_date,
    StockMovement.id,
)
Index(
    "ix_supplier_stock_event_date_row",
    SupplierStockSnapshot.event_date,
    SupplierStockSnapshot.row_id,
)


def model_to_dict(obj: SQLModel) -> dict[str, Any]:
    """Serialize SQLModel row to API JSON shape.

    - datetime fields rendered as ISO-8601 with 'Z' suffix.
    - JSON columns returned as dicts (already parsed by SQLAlchemy JSON type).
    - Internal `row_id` surrogate is stripped from facts/master-multi entities.
    """
    raw = obj.model_dump()
    out: dict[str, Any] = {}
    for key, value in raw.items():
        if key == "row_id":
            continue
        if isinstance(value, datetime):
            out[key] = value.strftime("%Y-%m-%dT%H:%M:%SZ")
        else:
            out[key] = value
    return out


def parse_iso(s: str | None) -> datetime | None:
    if not s:
        return None
    s = s.replace("Z", "+00:00")
    try:
        dt = datetime.fromisoformat(s)
    except ValueError:
        return None
    if dt.tzinfo is not None:
        dt = dt.replace(tzinfo=None)
    return dt


def parse_date(s: str | None) -> datetime | None:
    if not s:
        return None
    try:
        return datetime.strptime(s, "%Y-%m-%d")
    except ValueError:
        return parse_iso(s)


def jsonable(obj: Any) -> Any:
    """Best-effort json roundtrip for arbitrary nested values."""
    return json.loads(json.dumps(obj, default=str))
