"""Lightweight type/serialization helpers for mock-erp on PostgreSQL.

After the SQLite → Postgres migration we no longer use SQLModel/SQLAlchemy.
Rows are returned by psycopg as dicts; this module only carries:
  - the canonical list of master/facts entities (used by the seeder + routes),
  - small helpers (parse_iso, parse_date, row_to_response) to keep the JSON
    contract identical to the previous SQLite implementation.

Field shape mirrors the testdata/fixtures/*.json contract used by
internal/features/data_export/loader/erp_e_zoo_reader.go.
"""
from __future__ import annotations

import json
from datetime import date, datetime
from typing import Any

# Master entities and their (pk_column, since_column) for /api/v1/{entity}.
# updated_col=None means the entity has no time-based "since" filter.
MASTER_ENTITIES: dict[str, tuple[str, str | None]] = {
    "products":                          ("id",     "updated_at"),
    "product_barcodes":                  ("barcode", None),
    "category":                          ("id",     "updated_at"),
    "location":                          ("id",     "updated_at"),
    "supplier":                          ("id",     "updated_at"),
    "supply_spec":                       ("row_id", "valid_from"),
    "promo":                             ("id",     "updated_at"),
    "order_rule":                        ("id",     "valid_from"),
    "supply_plan":                       ("id",     "plan_date"),
    "master_change_log":                 ("row_id", "changed_at"),
    "store_assortment":                  ("row_id", "updated_at"),
    "store_assortment_lifecycle_events": ("row_id", "event_date"),
}

# Facts entities: (pk_column, date_column).
FACTS_ENTITIES: dict[str, tuple[str, str]] = {
    "receipt_line":            ("id",     "event_date"),
    "location_stock_snapshot": ("row_id", "event_date"),
    "stock_movement":          ("id",     "event_date"),
    "supplier_stock_snapshot": ("row_id", "event_date"),
}

# Tables wiped by /admin/seed/reset (full DELETE). Order matters only if there
# are FKs — here there are none, so any order works.
ALL_DATA_TABLES: list[str] = [
    # facts
    "receipt_line",
    "location_stock_snapshot",
    "stock_movement",
    "supplier_stock_snapshot",
    # master
    "store_assortment_lifecycle_events",
    "store_assortment",
    "master_change_log",
    "supply_plan",
    "order_rule",
    "promo",
    "supply_spec",
    "product_barcodes",
    "products",
    "supplier",
    "location",
    "category",
    # write side
    "received_orders",
]

# Columns selected when serving GET /api/v1/{entity}. Excludes internal row_id
# for entities that use surrogate PKs.
ENTITY_RESPONSE_COLUMNS: dict[str, list[str]] = {
    "products":                          ["id", "sku", "name", "category_id", "unit", "pack_size",
                                          "is_active", "attributes", "updated_at"],
    "product_barcodes":                  ["barcode", "product_id"],
    "category":                          ["id", "name", "path", "updated_at"],
    "location":                          ["id", "type", "name", "region", "updated_at"],
    "supplier":                          ["id", "name", "inn", "updated_at"],
    "supply_spec":                       ["product_id", "supplier_id", "pack_qty", "lead_time_days",
                                          "min_order_qty", "multiple", "valid_from"],
    "promo":                             ["id", "location_id", "product_id", "start_date",
                                          "end_date", "discount_pct", "updated_at"],
    "order_rule":                        ["id", "location_id", "rule_type", "payload", "valid_from"],
    "supply_plan":                       ["id", "location_id", "product_id", "supplier_id",
                                          "plan_date", "qty"],
    "master_change_log":                 ["entity", "entity_pk", "field", "old_value",
                                          "new_value", "changed_at"],
    "store_assortment":                  ["location_id", "product_id", "start_date", "is_active",
                                          "updated_at"],
    "store_assortment_lifecycle_events": ["location_id", "product_id", "event_type", "event_date",
                                          "payload"],
    "receipt_line":                      ["id", "receipt_id", "location_id", "product_id", "qty",
                                          "price", "event_time", "event_date", "payload"],
    "location_stock_snapshot":           ["event_date", "location_id", "product_id", "qty_on_hand",
                                          "qty_reserved", "as_of"],
    "stock_movement":                    ["id", "event_date", "event_time", "location_id",
                                          "product_id", "movement_type", "qty", "ref_id", "payload"],
    "supplier_stock_snapshot":           ["event_date", "supplier_id", "product_id", "qty", "as_of"],
}


def _serialize_value(value: Any) -> Any:
    if value is None:
        return None
    if isinstance(value, datetime):
        return value.strftime("%Y-%m-%dT%H:%M:%SZ")
    if isinstance(value, date):
        return value.strftime("%Y-%m-%d")
    return value


def row_to_response(row: dict, columns: list[str]) -> dict[str, Any]:
    """Render a psycopg dict-row as the public API JSON object.

    - datetimes → ISO-8601 with 'Z' suffix (UTC),
    - JSONB columns are already python objects (psycopg auto-decodes JSONB),
    - row_id is dropped (entity_response_columns never includes it).
    """
    return {col: _serialize_value(row.get(col)) for col in columns}


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
