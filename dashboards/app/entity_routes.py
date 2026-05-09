"""Универсальные маршруты для просмотра содержимого таблиц БД.

Endpoints:
    GET /entity/{schema}/{table}                  — список с поиском и пагинацией
    GET /entity/{schema}/{table}/detail?<pk_cols>  — карточка одной записи

Безопасность по SQL:
- schema/table из URL — только для lookup в `entity_registry.ENTITIES`;
  отсутствие → 404. Никогда не подставляются в SQL напрямую.
- Имена колонок и таблиц после lookup собираются через
  `psycopg.sql.Identifier`, значения — через `%s` placeholders.
"""
from __future__ import annotations

import logging
from pathlib import Path
from typing import Any
from urllib.parse import urlencode

from fastapi import APIRouter, HTTPException, Request
from fastapi.responses import HTMLResponse
from fastapi.templating import Jinja2Templates
from psycopg import sql as pg_sql
from psycopg.rows import dict_row

from app import db
from app.entity_registry import ENTITIES, EntityDef, get_entity, list_entities

logger = logging.getLogger(__name__)

router = APIRouter()

BASE_DIR = Path(__file__).resolve().parent
templates = Jinja2Templates(directory=str(BASE_DIR / "templates"))

DEFAULT_PER_PAGE = 50
MAX_PER_PAGE = 200


# ----- Helpers -----------------------------------------------------------------


def _qual(entity: EntityDef) -> pg_sql.Composable:
    """Безопасный квалифицированный идентификатор схема.таблица."""
    return pg_sql.SQL("{}.{}").format(
        pg_sql.Identifier(entity.schema),
        pg_sql.Identifier(entity.table),
    )


def _columns_sql(cols: tuple[str, ...]) -> pg_sql.Composable:
    return pg_sql.SQL(", ").join(pg_sql.Identifier(c) for c in cols)


def _where_search(entity: EntityDef, q: str) -> tuple[pg_sql.Composable, list[Any]]:
    """Build WHERE clause for ILIKE search across `search_columns`. Returns
    (composable_clause, params). Empty composable when q is empty."""
    if not q or not entity.search_columns:
        return pg_sql.SQL(""), []
    parts = [
        pg_sql.SQL("{}::text ILIKE %s").format(pg_sql.Identifier(c))
        for c in entity.search_columns
    ]
    pattern = f"%{q}%"
    params = [pattern for _ in entity.search_columns]
    clause = pg_sql.SQL(" WHERE ") + pg_sql.SQL(" OR ").join(parts)
    return clause, params


def _approx_count(entity: EntityDef) -> int:
    """Fast approximate row count from pg_class.reltuples (for partitioned/large)."""
    q = """
        SELECT COALESCE(GREATEST(SUM(reltuples)::bigint, 0), 0) AS n
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = %s
          AND (c.relname = %s
               OR c.relname LIKE %s)
          AND c.relkind IN ('r', 'p')
    """
    partition_pattern = f"{entity.table}_%"
    row = db.fetch_one(q, (entity.schema, entity.table, partition_pattern))
    if not row:
        return 0
    return int(row.get("n") or 0)


def _exact_count(entity: EntityDef, q: str) -> int:
    base = pg_sql.SQL("SELECT COUNT(*) AS n FROM ") + _qual(entity)
    where, params = _where_search(entity, q)
    sql_obj = base + where
    with db.get_conn() as conn, conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql_obj, params)
        row = cur.fetchone()
    if not row:
        return 0
    return int(row.get("n") or 0)


def _build_list_query(
    entity: EntityDef,
    q: str,
    limit: int,
    offset: int,
) -> tuple[pg_sql.Composable, list[Any]]:
    select = pg_sql.SQL("SELECT ") + _columns_sql(entity.list_columns)
    base = select + pg_sql.SQL(" FROM ") + _qual(entity)
    where, params = _where_search(entity, q)
    order = pg_sql.SQL(" ORDER BY ") + pg_sql.SQL(entity.order_by)  # registry-controlled
    tail = pg_sql.SQL(" LIMIT %s OFFSET %s")
    full = base + where + order + tail
    return full, params + [limit, offset]


def _build_detail_query(
    entity: EntityDef,
    pk_values: list[Any],
) -> pg_sql.Composable:
    where_parts = [
        pg_sql.SQL("{} = %s").format(pg_sql.Identifier(c)) for c in entity.pk
    ]
    return (
        pg_sql.SQL("SELECT * FROM ")
        + _qual(entity)
        + pg_sql.SQL(" WHERE ")
        + pg_sql.SQL(" AND ").join(where_parts)
        + pg_sql.SQL(" LIMIT 1")
    )


def _detail_url(entity: EntityDef, row: dict[str, Any]) -> str:
    """Construct /entity/{schema}/{table}/detail?<pk_cols=...> URL for a row."""
    pairs = [(c, row.get(c)) for c in entity.pk if row.get(c) is not None]
    if not pairs:
        return ""
    return (
        f"/entity/{entity.schema}/{entity.table}/detail?"
        + urlencode([(k, str(v)) for k, v in pairs])
    )


def _fetch_list(
    entity: EntityDef,
    q: str,
    limit: int,
    offset: int,
) -> list[dict[str, Any]]:
    sql_obj, params = _build_list_query(entity, q, limit, offset)
    with db.get_conn() as conn, conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql_obj, params)
        return list(cur.fetchall())


def _fetch_detail(
    entity: EntityDef,
    pk_values: list[Any],
) -> dict[str, Any] | None:
    sql_obj = _build_detail_query(entity, pk_values)
    with db.get_conn() as conn, conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql_obj, pk_values)
        row = cur.fetchone()
    return dict(row) if row else None


# ----- Routes -----------------------------------------------------------------


@router.get("/entity", response_class=HTMLResponse)
async def entity_index(request: Request) -> HTMLResponse:
    """Индексная страница реестра — список всех таблиц со ссылками."""
    by_schema: dict[str, list[EntityDef]] = {}
    for ent in list_entities():
        by_schema.setdefault(ent.schema, []).append(ent)
    for ents in by_schema.values():
        ents.sort(key=lambda e: e.friendly_name)
    return templates.TemplateResponse(
        "entity_index.html",
        {
            "request": request,
            "title": "Список всех сущностей",
            "by_schema": by_schema,
            "module": None,
            "prev": None,
            "next": None,
        },
    )


@router.get("/entity/{schema}/{table}", response_class=HTMLResponse)
async def entity_list(
    request: Request,
    schema: str,
    table: str,
    q: str = "",
    page: int = 1,
    per_page: int = DEFAULT_PER_PAGE,
) -> HTMLResponse:
    entity = get_entity(schema, table)
    if entity is None:
        raise HTTPException(status_code=404, detail=f"unknown entity {schema}.{table}")

    page = max(1, page)
    per_page = max(1, min(per_page, MAX_PER_PAGE))
    offset = (page - 1) * per_page

    try:
        rows = _fetch_list(entity, q, per_page, offset)
    except Exception as exc:  # noqa: BLE001
        logger.warning("entity_list query failed for %s.%s: %s",
                       schema, table, exc)
        rows = []

    if entity.large:
        total: int | None = _approx_count(entity) if not q else None
        total_label = "≈"
    else:
        try:
            total = _exact_count(entity, q)
            total_label = ""
        except Exception as exc:  # noqa: BLE001
            logger.warning("entity_list count failed for %s.%s: %s",
                           schema, table, exc)
            total = None
            total_label = ""

    has_next = len(rows) == per_page
    if total is not None:
        has_next = has_next and (offset + len(rows)) < total

    detail_urls = [_detail_url(entity, row) for row in rows]

    return templates.TemplateResponse(
        "entity_list.html",
        {
            "request": request,
            "title": f"{entity.friendly_name} — {entity.schema}.{entity.table}",
            "entity": entity,
            "rows": rows,
            "detail_urls": detail_urls,
            "q": q,
            "page": page,
            "per_page": per_page,
            "total": total,
            "total_label": total_label,
            "has_next": has_next,
            "module": None,
            "prev": None,
            "next": None,
        },
    )


@router.get("/entity/{schema}/{table}/detail", response_class=HTMLResponse)
async def entity_detail(
    request: Request,
    schema: str,
    table: str,
) -> HTMLResponse:
    entity = get_entity(schema, table)
    if entity is None:
        raise HTTPException(status_code=404, detail=f"unknown entity {schema}.{table}")

    qp = dict(request.query_params)
    missing = [c for c in entity.pk if not qp.get(c)]
    if missing:
        raise HTTPException(
            status_code=400,
            detail=f"missing PK params: {', '.join(missing)}",
        )
    pk_values: list[Any] = [qp[c] for c in entity.pk]

    try:
        row = _fetch_detail(entity, pk_values)
    except Exception as exc:  # noqa: BLE001
        logger.warning("entity_detail query failed for %s.%s: %s",
                       schema, table, exc)
        row = None

    if row is None:
        raise HTTPException(
            status_code=404,
            detail=f"row not found in {schema}.{table}",
        )

    fk_resolved: dict[str, str] = {}
    for col, (fk_schema, fk_table) in entity.fk_links.items():
        val = row.get(col)
        if val is None:
            continue
        target = ENTITIES.get((fk_schema, fk_table))
        if target is None or len(target.pk) != 1:
            continue
        pk_col = target.pk[0]
        fk_resolved[col] = (
            f"/entity/{fk_schema}/{fk_table}/detail?"
            + urlencode({pk_col: str(val)})
        )

    return templates.TemplateResponse(
        "entity_card.html",
        {
            "request": request,
            "title": f"{entity.friendly_name}: {' / '.join(str(v) for v in pk_values)}",
            "entity": entity,
            "row": row,
            "pk_values": pk_values,
            "fk_links": fk_resolved,
            "module": None,
            "prev": None,
            "next": None,
        },
    )
