"""Write side: receive POs from channel-router and expose verification list."""
from __future__ import annotations

import json
from datetime import datetime
from typing import Any

from fastapi import APIRouter, Depends, Query, Response, status
from psycopg.rows import dict_row

from app.auth import require_api_key
from app.db import get_pool
from app.models import parse_iso

router = APIRouter(prefix="/api/v1", dependencies=[Depends(require_api_key)])


@router.post("/orders", status_code=status.HTTP_201_CREATED)
def receive_order(body: dict[str, Any]) -> dict[str, str]:
    """Accept PO from channel-router. Body shape is loose to match adapter quirks."""
    po = body.get("po") if isinstance(body, dict) else None
    po_number: str | None = None
    supplier_id: str | None = None
    location_id: str | None = None
    if isinstance(po, dict):
        po_number = po.get("po_number")
        supplier_id = po.get("supplier_id")
        location_id = po.get("location_id")

    accepted_at = datetime.utcnow()
    raw_body = json.dumps(body, ensure_ascii=False, default=str)

    pool = get_pool()
    with pool.connection() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT COALESCE(MAX(id), 0) FROM received_orders")
            last_id = int(cur.fetchone()[0])
            external_ref = f"ERP-{last_id + 1:06d}"
            cur.execute(
                """
                INSERT INTO received_orders
                    (external_ref, po_number, supplier_id, location_id, accepted_at, raw_body)
                VALUES (%s, %s, %s, %s, %s, %s::jsonb)
                """,
                (external_ref, po_number, supplier_id, location_id, accepted_at, raw_body),
            )
        conn.commit()

    return {
        "external_ref": external_ref,
        "accepted_at": accepted_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
    }


@router.get("/orders/received")
def list_received(
    response: Response,
    since: str | None = Query(default=None),
) -> list[dict[str, Any]]:
    where = ""
    args: tuple = ()
    since_dt = parse_iso(since) if since else None
    if since_dt is not None:
        where = " WHERE accepted_at >= %s"
        args = (since_dt,)

    pool = get_pool()
    with pool.connection() as conn:
        with conn.cursor(row_factory=dict_row) as cur:
            cur.execute(
                f"""
                SELECT id, external_ref, po_number, supplier_id, location_id,
                       accepted_at, raw_body
                FROM received_orders{where}
                ORDER BY accepted_at
                """,
                args,
            )
            rows = list(cur.fetchall())

    out: list[dict[str, Any]] = []
    for r in rows:
        out.append({
            "id": r["id"],
            "external_ref": r["external_ref"],
            "po_number": r["po_number"],
            "supplier_id": r["supplier_id"],
            "location_id": r["location_id"],
            "accepted_at": r["accepted_at"].strftime("%Y-%m-%dT%H:%M:%SZ"),
            "raw_body": r["raw_body"],
        })
    response.headers["X-Total-Count"] = str(len(out))
    return out
