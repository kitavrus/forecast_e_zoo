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


# ----- Healthcheck -------------------------------------------------------------


@app.get("/healthz", response_class=JSONResponse)
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


# ----- Index -------------------------------------------------------------------


@app.get("/", response_class=HTMLResponse)
async def index(request: Request) -> HTMLResponse:
    cards: list[dict[str, Any]] = []
    for m in MODULES:
        metric = _index_metric_for(m["n"])
        cards.append({**m, "metric_label": metric[0], "metric_value": metric[1]})
    return templates.TemplateResponse(
        "index.html",
        {"request": request, "modules": cards, "title": "e_zoo pipeline dashboards"},
    )


def _index_metric_for(n: int) -> tuple[str, str]:
    """Return one headline metric for the index card of module N."""
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

    prev_m, next_m = _module_neighbours(1)
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[0],
            "title": "M1 Source Adapter",
            "input_title": "mock-erp REST API (16 entities)",
            "input_counts": input_counts,
            "output_title": "public.* tables (PostgreSQL)",
            "output_counts": output_counts,
            "samples": [
                {"title": "Recent products (top 10 by updated_at)", "rows": products},
                {"title": "Recent receipt_line (top 10 by event_time DESC)", "rows": receipts},
                {"title": "Recent loads (top 5)", "rows": recent_loads},
            ],
            "extras": [
                {"title": "Latest load run", "kv": _kv_from_row(latest_load)},
                {"title": "Snapshot pointer", "kv": _kv_from_row(pointer)},
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
            "module": MODULES[1],
            "title": "M2 ETL Validation",
            "input_title": "source-adapter HTTP API",
            "input_counts": input_counts,
            "output_title": "marts.* schema",
            "output_counts": output_counts,
            "samples": [
                {"title": "Top 10 mart_demand_history by qty_sold DESC", "rows": top_demand},
                {"title": "Top 10 mart_calculation_input by on_hand DESC", "rows": top_calc},
                {"title": "Recent etl_runs (top 5)", "rows": recent_runs},
            ],
            "extras": [
                {"title": "Latest etl_run", "kv": _kv_from_row(latest_run)},
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
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[2],
            "title": "M3 Data Marts API",
            "input_title": "marts.* (read-only role mart_reader)",
            "input_counts": input_counts,
            "output_title": "HTTP /v1/marts/* (data-marts service)",
            "output_counts": output_counts,
            "samples": samples + [
                {"title": "Recent committed etl_runs (top 10)", "rows": versions},
            ],
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
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[3],
            "title": "M4 KPI Calibration",
            "input_title": "marts.* (demand history + calc input + scorecard)",
            "input_counts": input_counts,
            "output_title": "kpi.kpi_snapshots + kpi.kpi_calibrations",
            "output_counts": output_counts,
            "samples": [
                {"title": "10 critical KPI (lowest values)", "rows": critical},
                {"title": "kpi_calibrations (top 10)", "rows": calibrations},
            ],
            "extras": extras,
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
    return templates.TemplateResponse(
        "module.html",
        {
            "request": request,
            "module": MODULES[4],
            "title": "M5 Forecast Engine",
            "input_title": "marts.* + kpi.kpi_snapshots (через DB read)",
            "input_counts": input_counts,
            "output_title": "forecast.* schema",
            "output_counts": output_counts,
            "samples": [
                {"title": "Top 10 forecasts by forecast_qty DESC", "rows": top_forecasts},
                {"title": "Recent forecast_runs (top 5)", "rows": recent_runs},
            ],
            "extras": extras,
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
            "module": MODULES[5],
            "title": "M6 Order Builder",
            "input_title": "forecast.replenishment_plans WHERE status='approved'",
            "input_counts": input_counts,
            "output_title": "orders.purchase_orders + orders.po_lines",
            "output_counts": output_counts,
            "samples": [
                {"title": "Recent purchase_orders (top 10)", "rows": recent_pos},
                {"title": "Recent po_lines (top 10)", "rows": recent_lines},
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
            "module": MODULES[6],
            "title": "M7 Channel Router",
            "input_title": "orders.purchase_orders WHERE status='ready_to_send'",
            "input_counts": input_counts,
            "output_title": "channels.send_attempts + POST к mock-erp",
            "output_counts": output_counts,
            "samples": [
                {"title": "Recent send_attempts (top 10)", "rows": recent_attempts},
                {"title": "supplier_channel_config (top 10)", "rows": supplier_configs},
                {"title": "mock-erp /api/v1/orders/received (top 5)", "rows": received_sample},
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
