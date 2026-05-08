"""FastAPI app: 8 HTML pages (index + 7 modules)."""
from __future__ import annotations

import logging
import os
import time
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Any

import httpx
import jwt as pyjwt
from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.templating import Jinja2Templates
from pydantic_settings import BaseSettings

from app import db, queries
from app.descriptions import MODULE_DESCRIPTIONS, PIPELINE_OVERVIEW
from app.mock_erp_client import MockErpClient

logger = logging.getLogger("dashboards")
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")


class Settings(BaseSettings):
    """Runtime config — env-driven."""

    DASHBOARDS_DSN: str = (
        "postgres://e_zoo_app:ezoo_app_dev@postgres:5432/source_adapter"
    )
    MOCK_ERP_URL: str = "http://mock-erp:8090"
    MOCK_ERP_API_KEY: str = "test-api-key"
    DATA_MARTS_URL: str = "http://data-marts:8082"
    JWT_SECRET: str = "dev-secret-change-in-prod"
    JWT_ROLE: str = "it-read"
    HTTP_TIMEOUT_SEC: float = 5.0


settings = Settings()

BASE_DIR = Path(__file__).resolve().parent
templates = Jinja2Templates(directory=str(BASE_DIR / "templates"))


def _make_jwt() -> str:
    """Mint HS256 JWT for data-marts (iss=role, sub=dashboards)."""
    now = int(time.time())
    payload = {
        "iss": settings.JWT_ROLE,
        "sub": "dashboards",
        "iat": now,
        "exp": now + 600,
    }
    return pyjwt.encode(payload, settings.JWT_SECRET, algorithm="HS256")


@asynccontextmanager
async def lifespan(_app: FastAPI):
    db.init_pool(settings.DASHBOARDS_DSN)
    try:
        yield
    finally:
        db.close_pool()


app = FastAPI(title="e_zoo dashboards", version="0.1.0", lifespan=lifespan)


# ----- Helpers -----------------------------------------------------------------

MODULES = [
    {"n": 0, "slug": "m0", "emoji": "🌱", "name": "Mock ERP (Source)",
     "flow": "16 entities → REST API (X-Total-Count) — корень pipeline"},
    {"n": 1, "slug": "m1", "emoji": "📥", "name": "Source Adapter",
     "flow": "mock-erp REST → public.* tables"},
    {"n": 2, "slug": "m2", "emoji": "🔄", "name": "ETL Validation",
     "flow": "source-adapter API → marts.* tables"},
    {"n": 3, "slug": "m3", "emoji": "📊", "name": "Data Marts API",
     "flow": "marts.* → /v1/marts/* HTTP API"},
    {"n": 4, "slug": "m4", "emoji": "📈", "name": "KPI Calibration",
     "flow": "marts.* → kpi.kpi_snapshots"},
    {"n": 5, "slug": "m5", "emoji": "🔮", "name": "Forecast Engine",
     "flow": "marts.* → forecast.forecasts + replenishment_plans"},
    {"n": 6, "slug": "m6", "emoji": "🛒", "name": "Order Builder",
     "flow": "approved plans → orders.purchase_orders"},
    {"n": 7, "slug": "m7", "emoji": "📤", "name": "Channel Router",
     "flow": "ready_to_send POs → mock-erp + channels.send_attempts"},
]


def _safe_count(table: str) -> int:
    return int(db.fetch_scalar(queries.count_sql(table), default=0) or 0)


def _by_status(rows: list[dict[str, Any]]) -> list[tuple[str, int]]:
    return [(str(r.get("status", "")), int(r.get("n", 0) or 0)) for r in rows]


def _module_neighbours(n: int) -> tuple[dict | None, dict | None]:
    prev_m = next((m for m in MODULES if m["n"] == n - 1), None)
    next_m = next((m for m in MODULES if m["n"] == n + 1), None)
    return prev_m, next_m


def _description_for(slug: str) -> dict[str, Any] | None:
    """Получить русскоязычное описание модуля из descriptions.py."""
    return MODULE_DESCRIPTIONS.get(slug)  # type: ignore[return-value]


# ----- Healthcheck -------------------------------------------------------------


@app.get("/healthz", response_class=JSONResponse)
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


# ----- Index -------------------------------------------------------------------


@app.get("/", response_class=HTMLResponse)
async def index(request: Request) -> HTMLResponse:
    cards: list[dict[str, Any]] = []
    # M0 metric — async fetch из mock-erp (products X-Total-Count).
    m0_products: str = "n/a"
    try:
        client = MockErpClient(
            settings.MOCK_ERP_URL,
            settings.MOCK_ERP_API_KEY,
            timeout=settings.HTTP_TIMEOUT_SEC,
        )
        cnt = await client.get_total_count("products")
        m0_products = str(cnt) if cnt is not None else "n/a"
    except Exception as exc:  # noqa: BLE001
        logger.warning("index M0 metric failed: %s", exc)

    for m in MODULES:
        if m["n"] == 0:
            metric = ("mock-erp products", m0_products)
        else:
            metric = _index_metric_for(m["n"])
        desc = MODULE_DESCRIPTIONS.get(m["slug"])
        card_desc = desc["card_short"] if desc else m["flow"]
        cards.append({
            **m,
            "metric_label": metric[0],
            "metric_value": metric[1],
            "card_desc": card_desc,
        })
    return templates.TemplateResponse(
        "index.html",
        {
            "request": request,
            "modules": cards,
            "title": "e_zoo pipeline dashboards",
            "pipeline_overview": PIPELINE_OVERVIEW,
        },
    )


def _index_metric_for(n: int) -> tuple[str, str]:
    """Return one headline metric for the index card of module N (sync — DB only).

    Для M0 метрика берётся из mock-erp (async) в самом index-handler;
    эта функция вызывается только для N >= 1.
    """
    try:
        if n == 1:
            return ("products в БД", str(_safe_count("products")))
        if n == 2:
            return ("etl_runs", str(_safe_count("marts.etl_runs")))
        if n == 3:
            return ("etl_runs (committed)",
                    str(int(db.fetch_scalar(
                        "SELECT COUNT(*) FROM marts.etl_runs WHERE status='committed'",
                        default=0,
                    ) or 0)))
        if n == 4:
            return ("kpi_snapshots", str(_safe_count("kpi.kpi_snapshots")))
        if n == 5:
            return ("forecasts", str(_safe_count("forecast.forecasts")))
        if n == 6:
            return ("purchase_orders", str(_safe_count("orders.purchase_orders")))
        if n == 7:
            return ("send_attempts", str(_safe_count("channels.send_attempts")))
    except Exception as exc:  # noqa: BLE001
        logger.warning("index metric failed for m%s: %s", n, exc)
    return ("rows", "0")


# ----- M0 Mock ERP (source) ---------------------------------------------------


def _format_loaded_status(source_cnt: int | None, pulled_cnt: int,
                          mvp_skipped: bool) -> tuple[str, str]:
    """Return (status_label, loss_str) for pipeline summary table."""
    if mvp_skipped:
        return ("MVP skip", "—")
    if source_cnt is None:
        return ("⚠️ source n/a", "—")
    if source_cnt == 0 and pulled_cnt == 0:
        return ("∅ both empty", "0")
    threshold = source_cnt * 0.95
    if pulled_cnt >= threshold:
        return ("✅", str(source_cnt - pulled_cnt))
    if pulled_cnt == 0:
        return ("⛔ not loaded", str(source_cnt))
    return ("⚠️ partial", str(source_cnt - pulled_cnt))


@app.get("/m0", response_class=HTMLResponse)
async def m0(request: Request) -> HTMLResponse:
    """Mock ERP source dashboard — shows pipeline-wide data flow."""
    client = MockErpClient(
        settings.MOCK_ERP_URL,
        settings.MOCK_ERP_API_KEY,
        timeout=settings.HTTP_TIMEOUT_SEC,
    )

    # 1. Initial inventory in mock-erp — 16 entities X-Total-Count.
    source_counts = await client.get_total_counts(queries.MOCK_ERP_ENTITIES)

    # 2. Pulled counts from public.*.
    pipeline_rows: list[dict[str, Any]] = []
    for entity in queries.MOCK_ERP_ENTITIES:
        public_table = queries.ENTITY_TO_PUBLIC_TABLE.get(entity)
        mvp_skipped = public_table is None
        pulled = 0 if mvp_skipped else _safe_count(public_table)
        src = source_counts.get(entity)
        status, loss = _format_loaded_status(src, pulled, mvp_skipped)
        pipeline_rows.append({
            "entity": entity,
            "source_count": str(src) if src is not None else "n/a",
            "public_table": public_table or "(MVP skip)",
            "pulled_count": str(pulled),
            "loaded": status,
            "loss": loss,
        })

    # 3. POs received from pipeline (via mock-erp).
    received_count = await client.get_received_orders_count()
    received_sample = await client.get_received_orders_sample(limit=10)

    # 4. Compare with our orders.purchase_orders (sent-status).
    po_sent = int(db.fetch_scalar(queries.M0_QUERIES["po_sent_count"], default=0) or 0)
    po_total = int(db.fetch_scalar(queries.M0_QUERIES["po_total_count"], default=0) or 0)

    received_str = str(received_count) if received_count is not None else "n/a"
    match_label = "—"
    if received_count is not None:
        if received_count == po_sent:
            match_label = f"✅ совпадает ({received_count})"
        else:
            match_label = f"⚠️ mismatch (mock-erp={received_count} vs sent={po_sent})"

    # 5. Last-run timestamps for end-to-end visibility.
    last_load = db.fetch_one(queries.M0_QUERIES["last_load_committed"]) or {}
    last_etl = db.fetch_one(queries.M0_QUERIES["last_etl_committed"]) or {}
    last_fc = db.fetch_one(queries.M0_QUERIES["last_forecast_run"]) or {}
    last_send = db.fetch_one(queries.M0_QUERIES["last_send_attempt"]) or {}

    # ASCII flow diagram with real numbers.
    products_src = source_counts.get("products")
    receipt_src = source_counts.get("receipt_line")
    flow_diagram = (
        "mock-erp (source)\n"
        f"  ├── 16 entities (e.g. products={products_src or '?'}, "
        f"receipt_line={receipt_src or '?'})\n"
        f"  │       ↓ source-adapter (M1) → public.*  "
        f"[products={_safe_count('products')}, receipt_line={_safe_count('receipt_line')}]\n"
        f"  │       ↓ etl (M2) → marts.*              "
        f"[demand={_safe_count('marts.mart_demand_history')}, "
        f"calc={_safe_count('marts.mart_calculation_input')}]\n"
        f"  │       ↓ kpi (M4) → kpi.kpi_snapshots    "
        f"[{_safe_count('kpi.kpi_snapshots')}]\n"
        f"  │       ↓ forecast (M5) → forecast.*      "
        f"[{_safe_count('forecast.forecasts')} forecasts, "
        f"{_safe_count('forecast.replenishment_plans')} plans]\n"
        f"  │       ↓ order-builder (M6) → orders.*   "
        f"[{po_total} POs]\n"
        f"  │       ↓ channel-router (M7) → mock-erp\n"
        f"  └── ← received_orders ←                   "
        f"[mock-erp got: {received_str}]\n"
    )

    input_counts: list[tuple[str, str]] = [
        (entity, str(cnt) if cnt is not None else "n/a")
        for entity, cnt in source_counts.items()
    ]

    output_counts: list[tuple[str, str]] = [
        ("orders.purchase_orders (всего)", str(po_total)),
        ("orders.purchase_orders (sent/ready/ack)", str(po_sent)),
        ("mock-erp /api/v1/orders/received", received_str),
        ("⇄ match", match_label),
    ]

    extras = [
        {"title": "End-to-end data flow (с реальными счётчиками)",
         "pre": flow_diagram},
        {"title": "Last successful runs", "kv": [
            ("last loads.committed_at",
             str(last_load.get("ts") or "—")),
            ("last marts.etl_runs.finished_at (committed)",
             str(last_etl.get("ts") or "—")),
            ("last forecast.forecast_runs.finished_at (completed)",
             str(last_fc.get("ts") or "—")),
            ("last channels.send_attempts.finished_at (success)",
             str(last_send.get("ts") or "—")),
        ]},
    ]

    samples = [
        {"title": "Pipeline movement summary (entity × source × pulled × loss)",
         "caption": (
             "Сравнение количества сущностей в источнике (mock-erp) и в "
             "public.* PostgreSQL после загрузки M1. Loss = source − pulled. "
             "MVP skip — сущность пока не реплицируется в M1."),
         "rows": pipeline_rows},
        {"title": f"Received POs sample (top {len(received_sample)})",
         "caption": (
             f"Последние {len(received_sample)} заказов, которые mock-erp принял "
             "от Channel Router (M7) через POST /api/v1/orders. Замыкает loop "
             "pipeline."),
         "rows": received_sample},
    ]

    prev_m, next_m = _module_neighbours(0)
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[0],
            "description": _description_for("m0"),
            "title": "M0 Mock ERP (Source)",
            "input_title": "mock-erp /api/v1/{entity} — initial inventory (16 entities)",
            "input_summary": (
                "Mock-erp генерирует данные сам (Faker, 90 дней истории) и не "
                "имеет внешних входов. Также принимает входящие заказы от M7 "
                "в POST /api/v1/orders."),
            "input_counts": input_counts,
            "output_title": "orders.purchase_orders ↔ mock-erp /orders/received",
            "output_summary": (
                "Mock-erp отдаёт 16 типов сущностей через REST для M1 и "
                "принимает обратно заказы от M7. Match-проверка ниже сверяет "
                "число sent POs с числом полученных в mock-erp."),
            "output_counts": output_counts,
            "samples": samples,
            "extras": extras,
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- M1 ---------------------------------------------------------------------


@app.get("/m1", response_class=HTMLResponse)
async def m1(request: Request) -> HTMLResponse:
    # Input — mock-erp counts via HTTP
    input_counts: list[tuple[str, str]] = []
    async with httpx.AsyncClient(timeout=settings.HTTP_TIMEOUT_SEC) as client:
        for entity in queries.MOCK_ERP_ENTITIES:
            url = f"{settings.MOCK_ERP_URL}/api/v1/{entity}"
            try:
                r = await client.get(
                    url,
                    headers={"X-API-Key": settings.MOCK_ERP_API_KEY},
                    params={"limit": 1},
                )
                total = r.headers.get("X-Total-Count")
                if total is None and r.status_code == 200:
                    total = str(len(r.json()) if isinstance(r.json(), list) else 0)
                input_counts.append((entity, total or "?"))
            except Exception as exc:  # noqa: BLE001
                logger.warning("mock-erp fetch %s failed: %s", entity, exc)
                input_counts.append((entity, "n/a"))

    output_counts = [(t, str(_safe_count(t))) for t in queries.M1_PUBLIC_TABLES]
    latest_load = db.fetch_one(queries.M1_QUERIES["latest_load"])
    pointer = db.fetch_one(queries.M1_QUERIES["snapshot_pointer"])
    products = db.fetch_all(queries.M1_QUERIES["recent_products"])
    receipts = db.fetch_all(queries.M1_QUERIES["recent_receipts"])
    recent_loads = db.fetch_all(queries.M1_QUERIES["recent_loads"])

    # Source delta: сколько данных из mock-erp прошло через M1 в public.*.
    delta_kv: list[tuple[str, str]] = []
    for entity in queries.M1_PUBLIC_TABLES:
        # entity name == public table name для M1 (см. ENTITY_TO_PUBLIC_TABLE).
        src = next((c for e, c in input_counts if e == entity), None)
        pulled = _safe_count(entity)
        if src is None or src in ("?", "n/a"):
            delta_kv.append((entity, f"public={pulled} / source=n/a"))
            continue
        try:
            src_int = int(src)
        except (TypeError, ValueError):
            delta_kv.append((entity, f"public={pulled} / source={src}"))
            continue
        loss = src_int - pulled
        sign = "✅" if loss == 0 else ("⚠️" if pulled > 0 else "⛔")
        delta_kv.append(
            (entity, f"{sign} public={pulled} / source={src_int} (loss={loss})"),
        )

    prev_m, next_m = _module_neighbours(1)
    pulled_total = sum(int(v) for _, v in output_counts if str(v).isdigit())
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[1],
            "description": _description_for("m1"),
            "title": "M1 Source Adapter",
            "input_title": "mock-erp REST API (16 entities)",
            "input_summary": (
                "Cron 02:00 ходит за 16 сущностями в mock-erp по HTTP "
                "(GET /api/v1/{entity}, X-API-Key). В таблице ниже — что есть "
                "в источнике в данный момент."),
            "input_counts": input_counts,
            "output_title": "public.* tables (PostgreSQL)",
            "output_summary": (
                f"После последнего успешного load M1 положил {pulled_total:,} "
                "строк во все public.* таблицы (см. разбивку ниже). Snapshot "
                "pointer flip-нут атомарно — потребители видят консистентный "
                "снимок."),
            "output_counts": output_counts,
            "samples": [
                {"title": "Recent products (top 10 by updated_at)",
                 "caption": "10 последних загруженных продуктов в public.products (по полю updated_at).",
                 "rows": products},
                {"title": "Recent receipt_line (top 10 by event_time DESC)",
                 "caption": "10 последних строк продаж в public.receipt_line (партиционированная таблица фактов).",
                 "rows": receipts},
                {"title": "Recent loads (top 5)",
                 "caption": "5 последних запусков load-джобы M1 со статусом и длительностью.",
                 "rows": recent_loads},
            ],
            "extras": [
                {"title": "Source delta — сколько прошло через M1 (mock-erp → public.*)",
                 "caption": (
                     "Разница между числом строк в mock-erp и в public.*. "
                     "✅ — потерь нет, ⚠️ — частичная загрузка, ⛔ — таблица "
                     "не загружалась (или MVP skip)."),
                 "kv": delta_kv},
                {"title": "Latest load run",
                 "caption": "Последний run load-джобы M1 (status, started_at, finished_at, lines_total/failed).",
                 "kv": _kv_from_row(latest_load)},
                {"title": "Snapshot pointer",
                 "caption": "Текущий snapshot_pointer — current_load_id, на который смотрят downstream-консумеры (M2).",
                 "kv": _kv_from_row(pointer)},
            ],
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- M2 ---------------------------------------------------------------------


@app.get("/m2", response_class=HTMLResponse)
async def m2(request: Request) -> HTMLResponse:
    input_counts = [
        ("/v1/products", "via source-adapter HTTP"),
        ("/v1/receipt_line", "via source-adapter HTTP"),
        ("source-adapter port", "8080"),
        ("(см. M1 для деталей источника)", "→ /m1"),
    ]
    output_counts = [(f"marts.{t}", str(_safe_count(f"marts.{t}"))) for t in queries.M2_MARTS_TABLES]
    output_counts.append(("marts.reject_log", str(_safe_count("marts.reject_log"))))

    latest_run = db.fetch_one(queries.M2_QUERIES["latest_run"])
    top_demand = db.fetch_all(queries.M2_QUERIES["top_demand"])
    top_calc = db.fetch_all(queries.M2_QUERIES["top_calc_input"])
    recent_runs = db.fetch_all(queries.M2_QUERIES["recent_runs"])

    prev_m, next_m = _module_neighbours(2)
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[2],
            "description": _description_for("m2"),
            "title": "M2 ETL Validation",
            "input_title": "source-adapter HTTP API",
            "input_summary": (
                "Cron 02:30 ходит за 16 сущностями в API M1 (NDJSON streaming, "
                "JWT с ролью x-flow-etl). Все берутся из одного "
                "source_load_id для атомарного snapshot."),
            "input_counts": input_counts,
            "output_title": "marts.* schema",
            "output_summary": (
                "После успешной валидации построены 5 mart-таблиц + reject_log. "
                "Atomic flip всех mart выполнен в одной транзакции — "
                "потребители (M3, M4, M5) видят консистентный набор."),
            "output_counts": output_counts,
            "samples": [
                {"title": "Top 10 mart_demand_history by qty_sold DESC",
                 "caption": (
                     "Топ-10 строк mart_demand_history с наибольшим qty_sold. "
                     "Используется Forecast (M5) для построения SMA30."),
                 "rows": top_demand},
                {"title": "Top 10 mart_calculation_input by on_hand DESC",
                 "caption": (
                     "Топ-10 строк mart_calculation_input с наибольшим "
                     "on_hand (текущий остаток). Pre-resolved supply_spec и "
                     "order_rule готовы для калькулятора M5."),
                 "rows": top_calc},
                {"title": "Recent etl_runs (top 5)",
                 "caption": "5 последних запусков ETL-джобы (status, source_load_id, finished_at, длительность).",
                 "rows": recent_runs},
            ],
            "extras": [
                {"title": "Latest etl_run",
                 "caption": "Последний run M2 — status, source_load_id (который снимок M1 он читал), длительность.",
                 "kv": _kv_from_row(latest_run)},
            ],
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- M3 ---------------------------------------------------------------------


@app.get("/m3", response_class=HTMLResponse)
async def m3(request: Request) -> HTMLResponse:
    input_counts = [(f"marts.{t}", str(_safe_count(f"marts.{t}"))) for t in queries.M2_MARTS_TABLES]

    output_counts: list[tuple[str, str]] = []
    samples: list[dict[str, Any]] = []
    token = _make_jwt()
    headers = {"Authorization": f"Bearer {token}"}

    async with httpx.AsyncClient(timeout=settings.HTTP_TIMEOUT_SEC) as client:
        for path_label, url in [
            ("GET /v1/marts", f"{settings.DATA_MARTS_URL}/v1/marts"),
            ("GET /v1/marts/mart_demand_history/version",
             f"{settings.DATA_MARTS_URL}/v1/marts/mart_demand_history/version"),
            ("GET /v1/marts/mart_calculation_input/version",
             f"{settings.DATA_MARTS_URL}/v1/marts/mart_calculation_input/version"),
        ]:
            try:
                r = await client.get(url, headers=headers)
                output_counts.append((path_label, f"{r.status_code} ({len(r.content)} B)"))
            except Exception as exc:  # noqa: BLE001
                output_counts.append((path_label, f"n/a ({exc.__class__.__name__})"))

        for mart in ("mart_demand_history", "mart_calculation_input"):
            url = f"{settings.DATA_MARTS_URL}/v1/marts/{mart}"
            try:
                r = await client.get(url, headers=headers, params={"limit": 5})
                if r.status_code == 200:
                    body = r.json()
                    rows = body.get("rows", body) if isinstance(body, dict) else body
                    samples.append({"title": f"GET /v1/marts/{mart}?limit=5", "rows": rows[:5]})
                else:
                    samples.append({"title": f"GET /v1/marts/{mart}?limit=5",
                                    "rows": [{"status": r.status_code,
                                              "body_preview": r.text[:200]}]})
            except Exception as exc:  # noqa: BLE001
                samples.append({"title": f"GET /v1/marts/{mart}?limit=5",
                                "rows": [{"error": str(exc)}]})

    versions = db.fetch_all(queries.M3_QUERIES["marts_versions"])

    prev_m, next_m = _module_neighbours(3)
    samples_with_captions: list[dict[str, Any]] = []
    for s in samples:
        s_with = dict(s)
        s_with.setdefault(
            "caption",
            "Реальный ответ data-marts API через JWT it-read (limit=5).",
        )
        samples_with_captions.append(s_with)
    samples_with_captions.append({
        "title": "Recent committed etl_runs (top 10)",
        "caption": (
            "10 последних committed etl_runs — каждая запись соответствует "
            "версии mart, доступной через API."),
        "rows": versions,
    })
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[3],
            "description": _description_for("m3"),
            "title": "M3 Data Marts API",
            "input_title": "marts.* (read-only role mart_reader)",
            "input_summary": (
                "M3 не имеет своего ETL — только читает marts.* через DB role "
                "mart_reader. Cache 60s для current snapshot уменьшает "
                "повторные запросы по той же версии."),
            "input_counts": input_counts,
            "output_title": "HTTP /v1/marts/* (data-marts service)",
            "output_summary": (
                "Live-проверка endpoint'ов data-marts: status code, размер "
                "тела ответа. NDJSON streaming с cursor-pagination + ETag."),
            "output_counts": output_counts,
            "samples": samples_with_captions,
            "extras": [],
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- M4 ---------------------------------------------------------------------


@app.get("/m4", response_class=HTMLResponse)
async def m4(request: Request) -> HTMLResponse:
    input_counts = [
        ("marts.mart_demand_history", str(_safe_count("marts.mart_demand_history"))),
        ("marts.mart_calculation_input", str(_safe_count("marts.mart_calculation_input"))),
        ("marts.mart_supplier_scorecard", str(_safe_count("marts.mart_supplier_scorecard"))),
    ]

    by_kpi = db.fetch_all(queries.M4_QUERIES["by_kpi"])
    cal_count = int(db.fetch_scalar(queries.M4_QUERIES["calibrations_count"], default=0) or 0)
    snap_count = int(db.fetch_scalar(queries.M4_QUERIES["snapshots_count"], default=0) or 0)
    output_counts: list[tuple[str, str]] = [
        ("kpi.kpi_snapshots (всего)", str(snap_count)),
        ("kpi.kpi_calibrations", str(cal_count)),
    ]
    for r in by_kpi:
        output_counts.append((f"  by kpi_name={r['kpi_name']}", str(r["n"])))

    critical = db.fetch_all(queries.M4_QUERIES["critical_low"])
    distribution = db.fetch_one(queries.M4_QUERIES["stock_days_distribution"]) or {}
    calibrations = db.fetch_all(queries.M4_QUERIES["calibrations"])

    extras = [
        {"title": "stock_days distribution", "kv": [
            ("< 7 days", str(distribution.get("lt_7", 0))),
            ("7..30 days", str(distribution.get("d7_30", 0))),
            ("30..90 days", str(distribution.get("d30_90", 0))),
            ("> 90 days", str(distribution.get("gt_90", 0))),
        ]},
    ]

    prev_m, next_m = _module_neighbours(4)
    extras_with_caption = list(extras)
    if extras_with_caption:
        extras_with_caption[0] = {
            **extras_with_caption[0],
            "caption": (
                "Распределение product×location пар по бакетам Stock Days. "
                "<7 дней — критично (риск out-of-stock), >90 дней — overstock."),
        }
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[4],
            "description": _description_for("m4"),
            "title": "M4 KPI Calibration",
            "input_title": "marts.* (demand history + calc input + scorecard)",
            "input_summary": (
                "Cron 04:00 читает напрямую из marts.* (без HTTP) — "
                "consistency snapshot гарантирована atomic flip M2."),
            "input_counts": input_counts,
            "output_title": "kpi.kpi_snapshots + kpi.kpi_calibrations",
            "output_summary": (
                "Считаются три KPI (OSA, OTIF, Stock Days) для каждой пары "
                "product×location. Hierarchical калибровки применяются по "
                "приоритету: pair → location → supplier → category → global."),
            "output_counts": output_counts,
            "samples": [
                {"title": "10 critical KPI (lowest values)",
                 "caption": (
                     "Топ-10 product×location пар с самыми низкими значениями "
                     "KPI — кандидаты на немедленное пополнение или внимание."),
                 "rows": critical},
                {"title": "kpi_calibrations (top 10)",
                 "caption": (
                     "10 активных калибровок: scope (global/category/...), "
                     "kpi_name, target value. Перебивают расчётные значения."),
                 "rows": calibrations},
            ],
            "extras": extras_with_caption,
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- M5 ---------------------------------------------------------------------


@app.get("/m5", response_class=HTMLResponse)
async def m5(request: Request) -> HTMLResponse:
    input_counts = [
        ("marts.mart_demand_history", str(_safe_count("marts.mart_demand_history"))),
        ("marts.mart_calculation_input", str(_safe_count("marts.mart_calculation_input"))),
        ("kpi.kpi_snapshots", str(_safe_count("kpi.kpi_snapshots"))),
    ]

    agg = db.fetch_one(queries.M5_QUERIES["forecasts_agg"]) or {}
    plans_by_status = db.fetch_all(queries.M5_QUERIES["plans_by_status"])
    plans_count = int(db.fetch_scalar(queries.M5_QUERIES["plans_count"], default=0) or 0)
    forecasts_count = int(db.fetch_scalar(queries.M5_QUERIES["forecasts_count"], default=0) or 0)

    output_counts: list[tuple[str, str]] = [
        ("forecast.forecasts", str(forecasts_count)),
        ("forecast.replenishment_plans", str(plans_count)),
        ("forecast.forecast_runs", str(_safe_count("forecast.forecast_runs"))),
    ]
    for r in plans_by_status:
        output_counts.append((f"  plans status={r['status']}", str(r["n"])))

    top_forecasts = db.fetch_all(queries.M5_QUERIES["top_forecasts"])
    latest_run = db.fetch_one(queries.M5_QUERIES["latest_run"])
    recent_runs = db.fetch_all(queries.M5_QUERIES["recent_runs"])

    extras = [
        {"title": "Forecasts aggregation", "kv": [
            ("total rows", str(agg.get("total", 0))),
            ("distinct (product, location)", str(agg.get("pairs", 0))),
            ("avg forecast_qty", str(agg.get("avg_qty", 0))),
            ("min forecast_qty", str(agg.get("min_qty", 0))),
            ("max forecast_qty", str(agg.get("max_qty", 0))),
        ]},
        {"title": "Latest forecast_run", "kv": _kv_from_row(latest_run)},
    ]

    prev_m, next_m = _module_neighbours(5)
    draft_count = sum(int(r["n"]) for r in plans_by_status if r.get("status") == "draft")
    extras_with_captions = [
        {
            **extras[0],
            "caption": (
                "Итоги по всем forecasts в forecast.forecasts: число строк, "
                "уникальных пар (product, location), статистика forecast_qty. "
                "Алгоритм: SMA30 × DOW × WOY на 60 дней вперёд."),
        },
        {
            **extras[1],
            "caption": (
                "Последний forecast_run — status, started_at, finished_at, "
                "сколько пар обработано."),
        },
    ]
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[5],
            "description": _description_for("m5"),
            "title": "M5 Forecast Engine",
            "input_title": "marts.* + kpi.kpi_snapshots (через DB read)",
            "input_summary": (
                "Cron 05:00 читает напрямую из marts.* и kpi.* через DB role "
                "(без HTTP — производительность важна на больших фан-аутах). "
                "Forecaster использует историю продаж за 90 дней."),
            "input_counts": input_counts,
            "output_title": "forecast.* schema",
            "output_summary": (
                f"Прогнозы записаны в forecast.forecasts ({forecasts_count} "
                f"строк), а replenishment_plans ({plans_count} планов, в т.ч. "
                f"{draft_count} в status=draft) ждут одобрения admin'ом перед "
                "конвертацией в Order Builder (M6)."),
            "output_counts": output_counts,
            "samples": [
                {"title": "Top 10 forecasts by forecast_qty DESC",
                 "caption": (
                     "Топ-10 прогнозов с наибольшим forecast_qty за 60 дней — "
                     "это пары product×location с самым высоким ожидаемым спросом."),
                 "rows": top_forecasts},
                {"title": "Recent forecast_runs (top 5)",
                 "caption": "5 последних запусков forecast-джобы с длительностью и числом пар.",
                 "rows": recent_runs},
            ],
            "extras": extras_with_captions,
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- M6 ---------------------------------------------------------------------


@app.get("/m6", response_class=HTMLResponse)
async def m6(request: Request) -> HTMLResponse:
    approved = int(db.fetch_scalar(queries.M6_QUERIES["approved_plans"], default=0) or 0)
    plans_total = _safe_count("forecast.replenishment_plans")
    input_counts = [
        ("forecast.replenishment_plans (всего)", str(plans_total)),
        ("forecast.replenishment_plans (status=approved)", str(approved)),
    ]

    po_status = db.fetch_all(queries.M6_QUERIES["po_by_status"])
    po_total = int(db.fetch_scalar(queries.M6_QUERIES["po_count"], default=0) or 0)
    lines_total = int(db.fetch_scalar(queries.M6_QUERIES["po_lines_count"], default=0) or 0)
    output_counts: list[tuple[str, str]] = [
        ("orders.purchase_orders (всего)", str(po_total)),
        ("orders.po_lines (всего)", str(lines_total)),
    ]
    for r in po_status:
        output_counts.append((f"  PO status={r['status']}", str(r["n"])))

    recent_pos = db.fetch_all(queries.M6_QUERIES["recent_pos"])
    recent_lines = db.fetch_all(queries.M6_QUERIES["recent_lines"])

    prev_m, next_m = _module_neighbours(6)
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[6],
            "description": _description_for("m6"),
            "title": "M6 Order Builder",
            "input_title": "forecast.replenishment_plans WHERE status='approved'",
            "input_summary": (
                f"Cron 06:00 подбирает approved-планы (всего одобрено: "
                f"{approved} из {plans_total}). Только approved конвертируются "
                "в полноценные purchase orders — draft и rejected пропускаются."),
            "input_counts": input_counts,
            "output_title": "orders.purchase_orders + orders.po_lines",
            "output_summary": (
                f"После последнего run: {po_total} purchase_orders, "
                f"{lines_total} po_lines (одна на каждую позицию заказа). "
                "PO numbers формата PO-YYYYMMDD-NNNNNN, delivery_date = "
                "today + supplier.lead_time_days."),
            "output_counts": output_counts,
            "samples": [
                {"title": "Recent purchase_orders (top 10)",
                 "caption": (
                     "10 последних созданных POs со статусом, supplier_id, "
                     "total_qty, delivery_date. Status workflow: draft → "
                     "ready_to_send → sent → confirmed_by_erp → received."),
                 "rows": recent_pos},
                {"title": "Recent po_lines (top 10)",
                 "caption": (
                     "10 последних строк заказов: product_id, qty, unit_price "
                     "(резолвится через pricing waterfall — products → "
                     "supplier.default → NULL)."),
                 "rows": recent_lines},
            ],
            "extras": [],
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- M7 ---------------------------------------------------------------------


@app.get("/m7", response_class=HTMLResponse)
async def m7(request: Request) -> HTMLResponse:
    ready = int(db.fetch_scalar(queries.M7_QUERIES["ready_to_send"], default=0) or 0)
    input_counts = [
        ("orders.purchase_orders (status=ready_to_send)", str(ready)),
        ("orders.purchase_orders (всего)", str(_safe_count("orders.purchase_orders"))),
    ]

    by_status = db.fetch_all(queries.M7_QUERIES["attempts_by_status"])
    attempts_total = int(db.fetch_scalar(queries.M7_QUERIES["attempts_count"], default=0) or 0)
    output_counts: list[tuple[str, str]] = [
        ("channels.send_attempts (всего)", str(attempts_total)),
    ]
    for r in by_status:
        output_counts.append((f"  status={r['status']}", str(r["n"])))

    # mock-erp received POs
    received_count = "n/a"
    received_sample: list[dict[str, Any]] = []
    async with httpx.AsyncClient(timeout=settings.HTTP_TIMEOUT_SEC) as client:
        try:
            r = await client.get(
                f"{settings.MOCK_ERP_URL}/api/v1/orders/received",
                headers={"X-API-Key": settings.MOCK_ERP_API_KEY},
                params={"limit": 5},
            )
            received_count = r.headers.get("X-Total-Count", "?")
            if r.status_code == 200:
                body = r.json()
                received_sample = body if isinstance(body, list) else body.get("items", [])
        except Exception as exc:  # noqa: BLE001
            logger.warning("mock-erp received fetch failed: %s", exc)
            received_count = "n/a"

    output_counts.append(("mock-erp /api/v1/orders/received (X-Total-Count)", str(received_count)))

    recent_attempts = db.fetch_all(queries.M7_QUERIES["recent_attempts"])
    supplier_configs = db.fetch_all(queries.M7_QUERIES["supplier_configs"])

    prev_m, next_m = _module_neighbours(7)
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[7],
            "description": _description_for("m7"),
            "title": "M7 Channel Router",
            "input_title": "orders.purchase_orders WHERE status='ready_to_send'",
            "input_summary": (
                f"Cron 06:30 подбирает POs со status=ready_to_send "
                f"({ready} штук готовы к отправке). Per-PO транзакция с "
                "SELECT FOR UPDATE — конкурентность безопасна."),
            "input_counts": input_counts,
            "output_title": "channels.send_attempts + POST к mock-erp",
            "output_summary": (
                f"Всего {attempts_total} попыток отправки в журнале "
                "send_attempts. Успешные (status=success) переводят PO в "
                "sent; mock-erp принимает заказы через POST /api/v1/orders "
                "и возвращает их через /orders/received."),
            "output_counts": output_counts,
            "samples": [
                {"title": "Recent send_attempts (top 10)",
                 "caption": (
                     "10 последних попыток отправки: po_id, status "
                     "(success/error), HTTP-код ответа ERP, длительность."),
                 "rows": recent_attempts},
                {"title": "supplier_channel_config (top 10)",
                 "caption": (
                     "Конфиги каналов отправки по supplier'ам: endpoint URL, "
                     "тип авторизации (api_key / jwt / oauth2), формат body."),
                 "rows": supplier_configs},
                {"title": "mock-erp /api/v1/orders/received (top 5)",
                 "caption": (
                     "Последние 5 принятых заказов в mock-erp — то, что "
                     "реально дошло до ERP клиента. Замыкает loop pipeline."),
                 "rows": received_sample},
            ],
            "extras": [],
            "prev": prev_m,
            "next": next_m,
        },
    )


# ----- Internal helpers --------------------------------------------------------


def _kv_from_row(row: dict[str, Any] | None) -> list[tuple[str, str]]:
    if not row:
        return [("(no rows)", "—")]
    return [(k, str(v) if v is not None else "—") for k, v in row.items()]
