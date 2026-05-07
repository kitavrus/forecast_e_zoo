"""Write side: receive POs from channel-router and expose verification list."""
from __future__ import annotations

import json
from datetime import datetime
from typing import Any

from fastapi import APIRouter, Depends, Query, Response, status
from sqlalchemy import func
from sqlmodel import Session, select

from app.auth import require_api_key
from app.db import get_session
from app.models import ReceivedOrder, parse_iso

router = APIRouter(prefix="/api/v1", dependencies=[Depends(require_api_key)])


def _next_external_ref(session: Session) -> str:
    last_id = session.exec(select(func.max(ReceivedOrder.id))).one()
    next_seq = (last_id or 0) + 1
    return f"ERP-{next_seq:06d}"


@router.post("/orders", status_code=status.HTTP_201_CREATED)
def receive_order(
    body: dict[str, Any],
    session: Session = Depends(get_session),
) -> dict[str, str]:
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
    external_ref = _next_external_ref(session)
    row = ReceivedOrder(
        external_ref=external_ref,
        po_number=po_number,
        supplier_id=supplier_id,
        location_id=location_id,
        accepted_at=accepted_at,
        raw_body=raw_body,
    )
    session.add(row)
    session.commit()
    session.refresh(row)
    return {
        "external_ref": external_ref,
        "accepted_at": accepted_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
    }


@router.get("/orders/received")
def list_received(
    response: Response,
    since: str | None = Query(default=None),
    session: Session = Depends(get_session),
) -> list[dict[str, Any]]:
    q = select(ReceivedOrder).order_by(ReceivedOrder.accepted_at)
    since_dt = parse_iso(since) if since else None
    if since_dt is not None:
        q = q.where(ReceivedOrder.accepted_at >= since_dt)
    rows = session.exec(q).all()
    out: list[dict[str, Any]] = []
    for r in rows:
        try:
            raw_obj = json.loads(r.raw_body)
        except json.JSONDecodeError:
            raw_obj = None
        out.append(
            {
                "id": r.id,
                "external_ref": r.external_ref,
                "po_number": r.po_number,
                "supplier_id": r.supplier_id,
                "location_id": r.location_id,
                "accepted_at": r.accepted_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
                "raw_body": raw_obj,
            }
        )
    response.headers["X-Total-Count"] = str(len(out))
    return out
