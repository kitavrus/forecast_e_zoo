"""SQL queries for each pipeline module dashboard."""
from __future__ import annotations

# 16 mock-erp entities — input источник для M1 (source-adapter).
MOCK_ERP_ENTITIES: list[str] = [
    "products",
    "product_barcodes",
    "category",
    "location",
    "supplier",
    "supply_spec",
    "promo",
    "order_rule",
    "supply_plan",
    "master_change_log",
    "store_assortment",
    "store_assortment_lifecycle_events",
    "receipt_line",
    "location_stock_snapshot",
    "stock_movement",
    "supplier_stock_snapshot",
]

# Public-таблицы куда source-adapter пишет committed snapshot.
# Phase 13: 16/16 entities — полный ingest всех master + facts.
M1_PUBLIC_TABLES: list[str] = [
    "products",
    "product_barcodes",
    "supplier",
    "location",
    "category",
    "order_rule",
    "supply_spec",
    "promo",
    "supply_plan",
    "master_change_log",
    "store_assortment",
    "store_assortment_lifecycle_events",
    "receipt_line",
    "location_stock_snapshot",
    "stock_movement",
    "supplier_stock_snapshot",
]

# Mapping mock-erp entity → имя таблицы в public.* (None = MVP не реализует).
# Используется на /m0 для сборки сводной pipeline-таблицы.
# Phase 13: все 16 entities загружаются — нет MVP-skip.
ENTITY_TO_PUBLIC_TABLE: dict[str, str | None] = {
    "products": "products",
    "product_barcodes": "product_barcodes",
    "category": "category",
    "location": "location",
    "supplier": "supplier",
    "supply_spec": "supply_spec",
    "promo": "promo",
    "order_rule": "order_rule",
    "supply_plan": "supply_plan",
    "master_change_log": "master_change_log",
    "store_assortment": "store_assortment",
    "store_assortment_lifecycle_events": "store_assortment_lifecycle_events",
    "receipt_line": "receipt_line",
    "location_stock_snapshot": "location_stock_snapshot",
    "stock_movement": "stock_movement",
    "supplier_stock_snapshot": "supplier_stock_snapshot",
}

# marts.* — output ETL.
M2_MARTS_TABLES: list[str] = [
    "mart_master_current",
    "mart_calculation_input",
    "mart_demand_history",
    "mart_kpi_daily",
    "mart_supplier_scorecard",
]

# ----- Helpers -----------------------------------------------------------------

def count_sql(table: str) -> str:
    return f"SELECT COUNT(*) AS n FROM {table}"


# ----- M1 Source Adapter -------------------------------------------------------

M1_QUERIES = {
    "latest_load": """
        SELECT load_id, status, started_at, committed_at, failed_at,
               source, lines_total, lines_failed
        FROM loads
        ORDER BY started_at DESC
        LIMIT 1
    """,
    "snapshot_pointer": """
        SELECT current_load_id, previous_load_id, committed_at
        FROM snapshot_pointer
        WHERE id = 1
    """,
    "recent_products": """
        SELECT id, sku, name, unit, is_active, updated_at
        FROM products
        ORDER BY updated_at DESC NULLS LAST
        LIMIT 10
    """,
    "recent_receipts": """
        SELECT receipt_id, product_id, location_id, qty, price, event_time
        FROM receipt_line
        ORDER BY event_time DESC NULLS LAST
        LIMIT 10
    """,
    "recent_loads": """
        SELECT load_id, status, started_at, committed_at,
               lines_total, lines_failed
        FROM loads
        ORDER BY started_at DESC
        LIMIT 5
    """,
}

# ----- M2 ETL ------------------------------------------------------------------

M2_QUERIES = {
    "latest_run": """
        SELECT id, status, kind, trigger, started_at, finished_at,
               source_load_id, lines_total, lines_failed
        FROM marts.etl_runs
        ORDER BY started_at DESC
        LIMIT 1
    """,
    "reject_count": "SELECT COUNT(*) AS n FROM marts.reject_log",
    "top_demand": """
        SELECT as_of_date, location_id, product_id, qty_sold,
               revenue_paid, transactions_count
        FROM marts.mart_demand_history
        ORDER BY qty_sold DESC NULLS LAST
        LIMIT 10
    """,
    "top_calc_input": """
        SELECT product_id, location_id, on_hand, in_transit,
               daily_demand, supplier_id, lead_time_days
        FROM marts.mart_calculation_input
        ORDER BY on_hand DESC NULLS LAST
        LIMIT 10
    """,
    "recent_runs": """
        SELECT id, status, started_at, finished_at,
               lines_total, lines_failed
        FROM marts.etl_runs
        ORDER BY started_at DESC
        LIMIT 5
    """,
}

# ----- M3 Data Marts (DB part — counts) ---------------------------------------
# marts.mart_versions может отсутствовать в текущей схеме — выбираем доступные
# индикаторы версионности из etl_runs (последние committed runs).

M3_QUERIES = {
    "marts_versions": """
        SELECT id AS run_id, status, target_mart, kind,
               started_at, committed_at, lines_total
        FROM marts.etl_runs
        WHERE status = 'committed'
        ORDER BY committed_at DESC NULLS LAST
        LIMIT 10
    """,
}

# ----- M4 KPI ------------------------------------------------------------------

M4_QUERIES = {
    "by_kpi": """
        SELECT kpi_name, COUNT(*) AS n,
               MIN(value) AS min_v, AVG(value)::numeric(18,4) AS avg_v,
               MAX(value) AS max_v
        FROM kpi.kpi_snapshots
        GROUP BY kpi_name
        ORDER BY kpi_name
    """,
    "calibrations_count": "SELECT COUNT(*) AS n FROM kpi.kpi_calibrations",
    "snapshots_count": "SELECT COUNT(*) AS n FROM kpi.kpi_snapshots",
    "critical_low": """
        SELECT as_of_date, kpi_name, scope_type, scope_id, value
        FROM kpi.kpi_snapshots
        ORDER BY value ASC NULLS LAST
        LIMIT 10
    """,
    "stock_days_distribution": """
        SELECT
            COUNT(*) FILTER (WHERE value < 7)                  AS lt_7,
            COUNT(*) FILTER (WHERE value >= 7 AND value < 30)  AS d7_30,
            COUNT(*) FILTER (WHERE value >= 30 AND value < 90) AS d30_90,
            COUNT(*) FILTER (WHERE value >= 90)                AS gt_90
        FROM kpi.kpi_snapshots
        WHERE kpi_name = 'stock_days'
    """,
    "calibrations": """
        SELECT kpi_name, scope_type, scope_id, params, updated_at
        FROM kpi.kpi_calibrations
        ORDER BY kpi_name, scope_type
        LIMIT 10
    """,
}

# ----- M5 Forecast -------------------------------------------------------------

M5_QUERIES = {
    "forecasts_agg": """
        SELECT COUNT(*) AS total,
               COUNT(DISTINCT (product_id, location_id)) AS pairs,
               COALESCE(AVG(forecast_qty), 0)::numeric(18,4) AS avg_qty,
               COALESCE(MIN(forecast_qty), 0)::numeric(18,4) AS min_qty,
               COALESCE(MAX(forecast_qty), 0)::numeric(18,4) AS max_qty
        FROM forecast.forecasts
    """,
    "top_forecasts": """
        SELECT product_id, location_id, forecast_date, forecast_qty,
               lower_bound, upper_bound, model_name
        FROM forecast.forecasts
        ORDER BY forecast_qty DESC NULLS LAST
        LIMIT 10
    """,
    "plans_by_status": """
        SELECT status, COUNT(*) AS n
        FROM forecast.replenishment_plans
        GROUP BY status
        ORDER BY status
    """,
    "latest_run": """
        SELECT id, status, started_at, finished_at, horizon_days,
               forecasts_count, lines_count, plans_count
        FROM forecast.forecast_runs
        ORDER BY started_at DESC
        LIMIT 1
    """,
    "recent_runs": """
        SELECT id, status, started_at, finished_at,
               forecasts_count, plans_count
        FROM forecast.forecast_runs
        ORDER BY started_at DESC
        LIMIT 5
    """,
    "plans_count": "SELECT COUNT(*) AS n FROM forecast.replenishment_plans",
    "forecasts_count": "SELECT COUNT(*) AS n FROM forecast.forecasts",
}

# ----- M6 Order Builder --------------------------------------------------------

M6_QUERIES = {
    "approved_plans": """
        SELECT COUNT(*) AS n
        FROM forecast.replenishment_plans
        WHERE status = 'approved'
    """,
    "po_by_status": """
        SELECT status, COUNT(*) AS n
        FROM orders.purchase_orders
        GROUP BY status
        ORDER BY status
    """,
    "po_count": "SELECT COUNT(*) AS n FROM orders.purchase_orders",
    "po_lines_count": "SELECT COUNT(*) AS n FROM orders.po_lines",
    "recent_pos": """
        SELECT po_number, plan_id, supplier_id, location_id, status,
               total_qty, total_amount, currency, created_at
        FROM orders.purchase_orders
        ORDER BY created_at DESC
        LIMIT 10
    """,
    "recent_lines": """
        SELECT id, po_id, product_id, qty, unit_price, line_amount, created_at
        FROM orders.po_lines
        ORDER BY created_at DESC
        LIMIT 10
    """,
}

# ----- M7 Channel Router -------------------------------------------------------

M7_QUERIES = {
    "ready_to_send": """
        SELECT COUNT(*) AS n
        FROM orders.purchase_orders
        WHERE status = 'ready_to_send'
    """,
    "attempts_by_status": """
        SELECT status, COUNT(*) AS n
        FROM channels.send_attempts
        GROUP BY status
        ORDER BY status
    """,
    "recent_attempts": """
        SELECT id, po_id, supplier_id, channel_type, status,
               http_status_code, started_at, finished_at, retry_count, external_ref
        FROM channels.send_attempts
        ORDER BY started_at DESC
        LIMIT 10
    """,
    "supplier_configs": """
        SELECT supplier_id, channel_type, endpoint_url,
               auth_mode, retry_max, is_active
        FROM channels.supplier_channel_config
        ORDER BY supplier_id
        LIMIT 10
    """,
    "attempts_count": "SELECT COUNT(*) AS n FROM channels.send_attempts",
}

# ----- M0 Mock ERP (source) ----------------------------------------------------

M0_QUERIES = {
    # PO sent vs received counter — для сравнения с mock-erp /orders/received.
    "po_sent_count": """
        SELECT COUNT(*) AS n
        FROM orders.purchase_orders
        WHERE status IN ('sent', 'ready_to_send', 'acknowledged')
    """,
    "po_total_count": "SELECT COUNT(*) AS n FROM orders.purchase_orders",
    # Last successful end-to-end run timestamps — для отображения "когда последний раз шёл pipeline".
    "last_load_committed": """
        SELECT MAX(committed_at) AS ts
        FROM loads
        WHERE status = 'committed'
    """,
    "last_etl_committed": """
        SELECT MAX(finished_at) AS ts
        FROM marts.etl_runs
        WHERE status = 'committed'
    """,
    "last_forecast_run": """
        SELECT MAX(finished_at) AS ts
        FROM forecast.forecast_runs
        WHERE status = 'completed'
    """,
    "last_send_attempt": """
        SELECT MAX(finished_at) AS ts
        FROM channels.send_attempts
        WHERE status = 'success'
    """,
}
