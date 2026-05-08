"""Описания сущностей для KV-таблиц на dashboards.

Используется на страницах /m{N} в карточках Input/Output, чтобы пояснить:
- что означает строка таблицы (что за сущность),
- что означает её значение (count/id/sum/HTTP-status и т.д.),
- зачем она нужна и где используется дальше по pipeline.

Структура:
    ENTITY_DESCRIPTIONS[key] = {
        "description": "что это такое (1–3 предложения)",
        "value":       "что означает число (count, sum, HTTP-status, ...)",
        "purpose":     "для чего нужно / где используется дальше",
        "links":       [(module_slug, anchor), ...]
                       — превратятся в [M2] [M5] cross-module ссылки.
    }

Ключи подобраны так, чтобы соответствовать строкам labels из main.py
(input_counts/output_counts). Lookup делает enrich_kv_rows() — он
сначала ищет полный label, затем извлекает имя сущности через
несколько эвристик (см. _normalize_key).
"""
from __future__ import annotations

import html
import re
from typing import Iterable

# ----------------------------------------------------------------------------
# Master entities (mock-erp / public.*)
# ----------------------------------------------------------------------------

ENTITY_DESCRIPTIONS: dict[str, dict] = {
    # ---- master entities (mock-erp /api/v1/* и public.*) -------------------
    "products": {
        "description": (
            "Каталог товаров (SKU). Каждый товар имеет уникальный UUID, "
            "артикул SKU-NNNNN, категорию, единицу измерения и pack_size."
        ),
        "value": "Количество товаров (count, integer).",
        "purpose": (
            "Базовая мастер-сущность. Используется во всех 7 модулях: ETL "
            "строит mart_master_current, forecast делает прогноз per product, "
            "orders формирует po_lines."
        ),
        "links": [
            ("m0", "products"),
            ("m1", "products"),
            ("m2", "mart_master_current"),
            ("m6", "po_lines"),
        ],
    },
    "product_barcodes": {
        "description": (
            "Связь штрих-кодов с продуктами. Один продукт может иметь "
            "несколько баркодов (EAN, UPC, internal)."
        ),
        "value": "Количество строк product_barcodes (count).",
        "purpose": (
            "Master-данные. В MVP только хранится; для POS-интеграции и "
            "checkpoint scanning в next iter."
        ),
        "links": [("m0", "product_barcodes"), ("m1", "product_barcodes")],
    },
    "category": {
        "description": (
            "Иерархия категорий товаров (Dry food, Wet food, Toys, и т.д.). "
            "Поддерживает parent_id для tree."
        ),
        "value": "Количество категорий (count). Типично 8–20.",
        "purpose": (
            "Используется для группировки KPI по категориям и calibration "
            "overrides на уровне категории."
        ),
        "links": [
            ("m0", "category"),
            ("m1", "category"),
            ("m4", "kpi_calibrations"),
        ],
    },
    "location": {
        "description": (
            "Локации (магазины, склады, DC). type ∈ {store, dc, warehouse}. "
            "Имеют city и address."
        ),
        "value": "Количество локаций (count). Pet retail обычно 5–30.",
        "purpose": (
            "Каждый прогноз и PO привязан к (product, location). Stock "
            "snapshots — по location."
        ),
        "links": [
            ("m0", "location"),
            ("m1", "location"),
            ("m4", "kpi_calibrations"),
            ("m5", "calculation_lines"),
            ("m6", "purchase_orders"),
        ],
    },
    "supplier": {
        "description": (
            "Поставщики товаров. Имеют code, контакт, currency, "
            "default_unit_price."
        ),
        "value": "Количество поставщиков (count). MVP: 15–50.",
        "purpose": (
            "Орчестрирует группировку plans → POs (один PO per supplier per "
            "location). Channel config (M7) задаёт endpoint отправки."
        ),
        "links": [
            ("m0", "supplier"),
            ("m1", "supplier"),
            ("m6", "purchase_orders"),
            ("m7", "supplier_channel_config"),
        ],
    },
    "supply_spec": {
        "description": (
            "Условия поставки (supplier × product × location): "
            "min_order_qty, purchase_price, currency, pack_size, "
            "lead_time_days."
        ),
        "value": (
            "Количество supply_spec строк (count). N×M (suppliers × "
            "products), при 100% покрытии = products × ~1.5 sup/product."
        ),
        "purpose": (
            "ETL резолвит supply_spec для каждой пары (product, location) → "
            "выбирает primary supplier с lead_time. Forecast Calculator "
            "использует lead_time_days в формуле reorder_point = "
            "safety_stock + lead_time × demand."
        ),
        "links": [
            ("m0", "supply_spec"),
            ("m1", "supply_spec"),
            ("m2", "mart_calculation_input"),
            ("m5", "forecasts"),
        ],
    },
    "promo": {
        "description": (
            "Промо-акции с date_from/date_to и discount_pct, применимы к "
            "набору products в location."
        ),
        "value": "Количество активных promo строк (count).",
        "purpose": (
            "В MVP читается, но не применяется к forecast. Promo lift "
            "modeling — next iter."
        ),
        "links": [("m0", "promo"), ("m1", "promo"), ("m5", "forecasts")],
    },
    "order_rule": {
        "description": (
            "Правила пополнения запасов. scope ∈ {global, category, "
            "supplier, location, product_location}, kind ∈ {reorder_point, "
            "minmax}, safety_stock_days, service_level_pct (95% default), "
            "override_moq."
        ),
        "value": (
            "Количество правил (count). При 100% MVP coverage = ~30 правил "
            "для всех scopes."
        ),
        "purpose": (
            "Hierarchical resolver выбирает наиболее специфичное правило "
            "для пары (product, location). Calculator (M5) использует "
            "safety_stock_days и service_level_pct в формуле "
            "safety_stock = z × stddev × √LT."
        ),
        "links": [
            ("m0", "order_rule"),
            ("m1", "order_rule"),
            ("m2", "mart_calculation_input"),
            ("m5", "calculation_lines"),
        ],
    },
    "supply_plan": {
        "description": (
            "Планируемые поставки от supplier на location: planned_qty, "
            "plan_date, status."
        ),
        "value": "Количество plans (count).",
        "purpose": (
            "Calculator вычитает уже on-order qty из target_stock: "
            "reorder_qty = target − stock − in_transit. В MVP читаются, "
            "но full integration — в next iter."
        ),
        "links": [
            ("m0", "supply_plan"),
            ("m1", "supply_plan"),
            ("m5", "calculation_lines"),
        ],
    },
    "master_change_log": {
        "description": (
            "Audit log изменений мастер-данных: entity, entity_id, "
            "change_type, changed_at, payload."
        ),
        "value": "Количество событий (count, append-only).",
        "purpose": (
            "Для bi-temporal recompute (Q-012, отложен в next iter). "
            "MVP: append-only, не используется downstream."
        ),
        "links": [("m0", "master_change_log"), ("m1", "master_change_log")],
    },
    "store_assortment": {
        "description": (
            "Какие продукты доступны в каких локациях. lifecycle_state "
            "∈ {active, inactive, phase_out}, effective_from/to."
        ),
        "value": (
            "Количество (product, location) пар активного assortment."
        ),
        "purpose": (
            "Определяет валидную область прогноза: forecast строится "
            "только для (product, location) в активном assortment."
        ),
        "links": [
            ("m0", "store_assortment"),
            ("m1", "store_assortment"),
            ("m2", "mart_master_current"),
        ],
    },
    "store_assortment_lifecycle_events": {
        "description": (
            "Изменения статуса assortment: started, stopped, "
            "promo_started, promo_stopped."
        ),
        "value": "Количество lifecycle событий (count, append-only).",
        "purpose": (
            "Audit trail для phase-in/phase-out продуктов. В MVP только "
            "хранится."
        ),
        "links": [
            ("m0", "store_assortment_lifecycle_events"),
            ("m1", "store_assortment_lifecycle_events"),
        ],
    },
    "receipt_line": {
        "description": (
            "Строки чеков POS-системы: продажа/возврат/корректировка "
            "с qty, price, event_time."
        ),
        "value": (
            "Количество строк чеков. На production: миллионы/день. "
            "В MVP: ~30K за 90 дней."
        ),
        "purpose": (
            "Главный источник demand для прогноза. ETL агрегирует по "
            "(product, location, as_of_date) в mart_demand_history. "
            "Forecast Engine берёт SMA30 из этой истории."
        ),
        "links": [
            ("m0", "receipt_line"),
            ("m1", "receipt_line"),
            ("m2", "mart_demand_history"),
        ],
    },
    "location_stock_snapshot": {
        "description": (
            "Snapshot остатков на location на конкретную дату: "
            "qty_on_hand, qty_reserved, as_of timestamp."
        ),
        "value": (
            "Количество snapshots. Weekly snapshots для последних 90 "
            "дней + daily для текущей недели."
        ),
        "purpose": (
            "Текущий on_hand для Calculator: reorder_qty = target − "
            "on_hand − in_transit. Без этой сущности pipeline не может "
            "рассчитать reorder_qty."
        ),
        "links": [
            ("m0", "location_stock_snapshot"),
            ("m1", "location_stock_snapshot"),
            ("m2", "mart_calculation_input"),
        ],
    },
    "stock_movement": {
        "description": (
            "Движения товаров: receipt, sale, adjustment, "
            "transfer_in/out, write_off с qty и ref_id."
        ),
        "value": "Количество движений. Append-only event log.",
        "purpose": (
            "Audit trail для inventory accounting. В MVP читается, но "
            "не используется в marts. Will be used для KPI 'shrinkage "
            "rate' и 'transfer efficiency' в next iter."
        ),
        "links": [("m0", "stock_movement"), ("m1", "stock_movement")],
    },
    "supplier_stock_snapshot": {
        "description": (
            "Остатки на стороне поставщика (visibility): qty available "
            "для каждого product."
        ),
        "value": "Количество snapshots от suppliers. Weekly.",
        "purpose": (
            "Для mart_supplier_scorecard (надёжность поставщика). В MVP "
            "используется частично — full implementation в next iter."
        ),
        "links": [
            ("m0", "supplier_stock_snapshot"),
            ("m1", "supplier_stock_snapshot"),
            ("m2", "mart_supplier_scorecard"),
        ],
    },
    # ---- marts.* ----------------------------------------------------------
    "mart_demand_history": {
        "description": (
            "Агрегированная история продаж per (product, location, "
            "as_of_date): qty_sold, qty_returned."
        ),
        "value": "Количество строк. ~365 дней × N активных пар.",
        "purpose": (
            "Главный input для Forecast (SMA30 + DOW + WOY) и KPI "
            "(stock_days = current / avg_demand_30d)."
        ),
        "links": [
            ("m2", "mart_demand_history"),
            ("m4", "kpi_snapshots"),
            ("m5", "forecasts"),
        ],
    },
    "mart_calculation_input": {
        "description": (
            "Готовые данные для Calculator: on_hand, in_transit, "
            "safety_stock, daily_demand, applicable_rule_id, "
            "supplier_id, lead_time_days. Денормализованная строка per "
            "(product, location)."
        ),
        "value": (
            "Количество строк = active (product, location) пары."
        ),
        "purpose": (
            "Калькулятор не делает join'ы — все нужные поля уже здесь. "
            "ETL собирает из 6 источников: products, supply_spec, "
            "order_rule, location_stock_snapshot, supply_plan, "
            "mart_demand_history."
        ),
        "links": [
            ("m2", "mart_calculation_input"),
            ("m5", "calculation_lines"),
        ],
    },
    "mart_kpi_daily": {
        "description": (
            "Дневной снимок KPI per (location, kpi_name, as_of_date): "
            "value (numeric)."
        ),
        "value": "Количество daily snapshots.",
        "purpose": (
            "Pre-aggregated KPI для быстрого запроса. KPI engine (M4) "
            "читает и применяет calibrations."
        ),
        "links": [("m2", "mart_kpi_daily"), ("m4", "kpi_snapshots")],
    },
    "mart_master_current": {
        "description": (
            "Current snapshot всех мастер-данных в одной таблице: "
            "entity_type + entity_id + payload (jsonb)."
        ),
        "value": (
            "Количество строк = sum(products + suppliers + locations + "
            "categories + ...)."
        ),
        "purpose": (
            "Унифицированный read для downstream модулей: 1 запрос "
            "вместо 5+ JOIN-ов."
        ),
        "links": [
            ("m2", "mart_master_current"),
            ("m3", "mart_master_current"),
            ("m5", "forecasts"),
            ("m6", "purchase_orders"),
        ],
    },
    "mart_supplier_scorecard": {
        "description": (
            "Rolling weekly scorecard per supplier: otif_pct, "
            "on_time_pct, in_full_pct, lead_time_avg."
        ),
        "value": (
            "Количество rows = suppliers × weeks (52 weeks default)."
        ),
        "purpose": (
            "Используется для KPI OTIF и для приоритизации supplier "
            "при формировании PO (если несколько suppliers могут "
            "поставить product)."
        ),
        "links": [
            ("m2", "mart_supplier_scorecard"),
            ("m4", "kpi_snapshots"),
            ("m6", "purchase_orders"),
        ],
    },
    "etl_runs": {
        "description": (
            "Registry ETL запусков: id (UUID), status "
            "(running/committed/failed), source_load_id, "
            "marts_summary (jsonb)."
        ),
        "value": "Количество ETL runs (append).",
        "purpose": (
            "Идемпотентность через advisory lock. Каждая mart-строка "
            "имеет etl_run_id для provenance."
        ),
        "links": [("m2", "mart_master_current")],
    },
    "reject_log": {
        "description": (
            "Строки, отклонённые cross-entity validation: severity, "
            "rule_id, message."
        ),
        "value": (
            "Количество отклонённых записей. >1% от total → ETL fail."
        ),
        "purpose": (
            "Quality gate. Если >1% строк отклонено — ETL run "
            "помечается failed, marts не flip-аются."
        ),
        "links": [("m2", "mart_master_current")],
    },
    # ---- kpi.* ------------------------------------------------------------
    "kpi_snapshots": {
        "description": (
            "KPI snapshot per (kpi_name, scope_type, scope_id, "
            "as_of_date): value (numeric)."
        ),
        "value": (
            "Количество. Per day × per scope × 3 KPI "
            "(OSA / OTIF / Stock Days)."
        ),
        "purpose": (
            "Forecast Engine может использовать KPI как входные сигналы. "
            "Внешние BI-системы читают через REST."
        ),
        "links": [("m4", "kpi_snapshots"), ("m5", "forecasts")],
    },
    "kpi_calibrations": {
        "description": (
            "Параметры калибровки KPI per scope: target_threshold "
            "(OSA), tolerance_window_hours (OTIF), avg_window_days "
            "(Stock Days)."
        ),
        "value": "Количество калибровок (count).",
        "purpose": (
            "Hierarchical override (product_location > location > "
            "supplier > category > global). Меняется через REST API "
            "без перезапуска."
        ),
        "links": [("m4", "kpi_calibrations")],
    },
    # ---- forecast.* -------------------------------------------------------
    "forecast_runs": {
        "description": (
            "Registry forecast запусков. snapshot_id ссылается на "
            "marts.etl_runs (snapshot consistency)."
        ),
        "value": "Количество runs (append).",
        "purpose": (
            "Гарантия, что прогноз построен на консистентном "
            "marts-snapshot."
        ),
        "links": [("m5", "forecast_runs")],
    },
    "forecasts": {
        "description": (
            "Прогноз спроса per (product, location, forecast_date): "
            "forecast_qty, lower_bound, upper_bound, model_name, "
            "confidence."
        ),
        "value": (
            "Количество прогнозов = pairs × horizon_days. Default "
            "horizon = 60."
        ),
        "purpose": (
            "Главный output модели. Constructor использует "
            "sum(forecast_qty в lead_time window) для "
            "lead_time_demand."
        ),
        "links": [("m5", "forecasts"), ("m6", "purchase_orders")],
    },
    "calculation_lines": {
        "description": (
            "Результат Calculator per (product, location): "
            "current_stock, reorder_point, reorder_qty."
        ),
        "value": "Количество lines = active pairs requiring action.",
        "purpose": (
            "Constructor группирует по supplier и формирует "
            "replenishment_plans. Calculation_lines содержат финальные "
            "qty per product."
        ),
        "links": [("m5", "calculation_lines"), ("m6", "po_lines")],
    },
    "replenishment_plans": {
        "description": (
            "Сгруппированные планы пополнения per supplier per "
            "location: total_qty, lines_count, status (draft/approved/"
            "cancelled/converted)."
        ),
        "value": (
            "Количество plans. После approve → конвертируются в POs (M6)."
        ),
        "purpose": (
            "Промежуточная сущность между Forecast и Order Builder. "
            "Admin одобряет вручную перед отправкой."
        ),
        "links": [
            ("m5", "replenishment_plans"),
            ("m6", "purchase_orders"),
        ],
    },
    # ---- orders.* ---------------------------------------------------------
    "purchase_orders": {
        "description": (
            "Purchase orders с уникальным po_number "
            "(PO-YYYYMMDD-NNNNNN), status workflow (draft → "
            "ready_to_send → sent → confirmed_by_erp → received | "
            "cancelled)."
        ),
        "value": (
            "Количество POs (count). Per supplier per day обычно 1."
        ),
        "purpose": (
            "Финальный артефакт перед отправкой в ERP клиента. "
            "Содержит pricing waterfall и delivery_date = "
            "order_date + lead_time."
        ),
        "links": [
            ("m6", "purchase_orders"),
            ("m7", "send_attempts"),
        ],
    },
    "po_lines": {
        "description": (
            "Позиции PO: product_id, qty, unit_price, line_amount, "
            "pricing_source ∈ {product, supplier_default, missing}."
        ),
        "value": "Количество строк = sum lines per PO.",
        "purpose": (
            "Полный детализированный список заказываемых SKU. Pricing "
            "waterfall: products.unit_price → "
            "supplier.default_unit_price → NULL."
        ),
        "links": [("m6", "po_lines")],
    },
    "po_status_history": {
        "description": (
            "Audit log status переходов: from_status → to_status, "
            "reason, changed_by, changed_at."
        ),
        "value": "Количество status events.",
        "purpose": (
            "Compliance trail для audit. Каждое изменение статуса "
            "логируется."
        ),
        "links": [("m6", "po_status_history")],
    },
    # ---- channels.* -------------------------------------------------------
    "supplier_channel_config": {
        "description": (
            "Per-supplier endpoint config: channel_type ∈ {erp_api, "
            "edi_x12, edi_edifact, 1c_xml, crm}, endpoint_url, "
            "auth_mode, timeout_sec, retry_max."
        ),
        "value": (
            "Количество активных configs = suppliers × "
            "supported_channels."
        ),
        "purpose": (
            "ChannelRouter резолвит config per supplier и формирует "
            "HTTP request body через formatter (JSON/EDI/XML)."
        ),
        "links": [
            ("m6", "purchase_orders"),
            ("m7", "supplier_channel_config"),
        ],
    },
    "send_attempts": {
        "description": (
            "Audit log отправок: status (pending/accepted/rejected), "
            "http_status_code, request/response body, retry_count, "
            "external_ref."
        ),
        "value": "Количество попыток (включая retries).",
        "purpose": (
            "Idempotency через external_ref. Если retry на уже "
            "принятом PO — возвращает existing attempt."
        ),
        "links": [
            ("m7", "send_attempts"),
            ("m0", "received_orders"),
        ],
    },
    # ---- HTTP / endpoints / синтетические строки ---------------------------
    "received_orders": {
        "description": (
            "POST /api/v1/orders в mock-erp принимает заказы от M7 "
            "(Channel Router). Mock-erp возвращает их через "
            "GET /orders/received."
        ),
        "value": (
            "Количество принятых заказов (X-Total-Count из mock-erp)."
        ),
        "purpose": (
            "Замыкает loop pipeline: orders.purchase_orders (M6) → "
            "send_attempts (M7) → mock-erp (M0). Match-проверка на /m0 "
            "сверяет sent vs received."
        ),
        "links": [
            ("m0", "received_orders"),
            ("m7", "send_attempts"),
        ],
    },
    "match": {
        "description": (
            "Сверка между числом отправленных POs (status=sent/ready/"
            "ack) и числом принятых mock-erp."
        ),
        "value": (
            "✅ — equal, ⚠️ — диф (потеря/задвоение), '—' — счётчик "
            "недоступен."
        ),
        "purpose": (
            "End-to-end health-check pipeline. Должен быть ✅ при "
            "штатной работе."
        ),
        "links": [("m6", "purchase_orders"), ("m7", "send_attempts")],
    },
    "/v1/products": {
        "description": (
            "HTTP endpoint M1 (source-adapter): NDJSON streaming, JWT "
            "с ролью x-flow-etl. M2 ETL читает все 16 entities через "
            "/v1/{entity}."
        ),
        "value": "Способ доступа (HTTP, не count).",
        "purpose": (
            "M2 ETL читает весь snapshot через эти endpoints в "
            "одном source_load_id."
        ),
        "links": [("m1", "products"), ("m2", "mart_master_current")],
    },
    "/v1/receipt_line": {
        "description": (
            "HTTP endpoint M1: NDJSON streaming чеков POS. Самый "
            "большой эндпоинт (миллионы строк на production)."
        ),
        "value": "Способ доступа (HTTP, не count).",
        "purpose": (
            "M2 ETL агрегирует receipt_line → mart_demand_history "
            "(per product/location/day)."
        ),
        "links": [("m1", "receipt_line"), ("m2", "mart_demand_history")],
    },
    "source-adapter port": {
        "description": (
            "Сетевой порт docker-сервиса source-adapter (M1) — "
            "8080. M2 ETL запрашивает по DNS-имени контейнера."
        ),
        "value": "Номер TCP-порта (8080).",
        "purpose": (
            "Контейнерная сеть compose. Из M2 ETL: "
            "http://source-adapter:8080/v1/{entity}."
        ),
        "links": [("m1", "products")],
    },
}


# ----------------------------------------------------------------------------
# Lookup helpers
# ----------------------------------------------------------------------------

# Regex для извлечения имени entity из произвольных labels вроде
# "marts.mart_demand_history", "kpi.kpi_snapshots (всего)",
# "orders.purchase_orders (status=ready_to_send)",
# "  by kpi_name=osa", "GET /v1/marts/mart_demand_history".
_SCHEMA_PREFIX_RE = re.compile(
    r"^(marts|public|kpi|forecast|orders|channels)\.([a-z_]+)"
)
_HTTP_MART_RE = re.compile(r"/v1/marts/([a-z_]+)")
_BARE_ENTITY_RE = re.compile(r"^([a-z_]+)\b")


def _normalize_key(label: str) -> str | None:
    """Извлечь канонический ключ ENTITY_DESCRIPTIONS из произвольного label.

    Возвращает None, если описание не найдено.
    """
    if not label:
        return None

    # 0. Direct hit (для "/v1/products", "match" и т.п.).
    if label in ENTITY_DESCRIPTIONS:
        return label

    stripped = label.strip()
    if stripped in ENTITY_DESCRIPTIONS:
        return stripped

    # 1. "marts.mart_demand_history" / "kpi.kpi_snapshots" / "orders.po_lines".
    m = _SCHEMA_PREFIX_RE.match(stripped)
    if m:
        candidate = m.group(2)
        if candidate in ENTITY_DESCRIPTIONS:
            return candidate

    # 2. "GET /v1/marts/mart_demand_history" — берём mart name.
    m = _HTTP_MART_RE.search(stripped)
    if m:
        candidate = m.group(1)
        if candidate in ENTITY_DESCRIPTIONS:
            return candidate

    # 3. Спецкейсы из main.py.
    if "orders/received" in stripped:
        return "received_orders"
    if "match" in stripped.lower():
        return "match"

    # 4. Bare entity name (возможно с trailing " (всего)" / "(status=...)").
    m = _BARE_ENTITY_RE.match(stripped)
    if m:
        candidate = m.group(1)
        if candidate in ENTITY_DESCRIPTIONS:
            return candidate

    # 5. Indented подкатегории вроде "  by kpi_name=osa", "  PO status=sent",
    #    "  status=success", "  plans status=draft" — это разбивки родителя,
    #    отдельного описания им не нужно.
    return None


def _build_links_html(links: Iterable[tuple[str, str]]) -> str:
    """Сформировать HTML cross-module ссылок [M2] [M5] и т.д."""
    parts: list[str] = []
    seen: set[str] = set()
    for module_slug, anchor in links:
        if module_slug in seen:
            continue
        seen.add(module_slug)
        # /m2#mart_demand_history.
        href = f"/{module_slug}#{anchor}" if anchor else f"/{module_slug}"
        label = module_slug.upper()
        parts.append(
            f'<a class="cross-link" href="{html.escape(href, quote=True)}" '
            f'title="{html.escape(anchor, quote=True)}">[{html.escape(label)}]</a>'
        )
    return " ".join(parts)


def render_description_html(label: str) -> tuple[str, str]:
    """Вернуть (description_html, anchor_id) для строки KV-таблицы.

    description_html — готовый innerHTML 3-й ячейки (description, value,
    purpose, links). Если описание не найдено — возвращает '—'.

    anchor_id — нормализованный ключ entity (для атрибута id у <tr>),
    либо пустая строка если ключ не определён.
    """
    key = _normalize_key(label)
    if key is None:
        return ("<span class=\"desc-missing\">—</span>", "")

    desc = ENTITY_DESCRIPTIONS[key]
    desc_text = html.escape(desc.get("description", ""))
    value_text = html.escape(desc.get("value", ""))
    purpose_text = html.escape(desc.get("purpose", ""))
    links_html = _build_links_html(desc.get("links", []))

    parts: list[str] = []
    if desc_text:
        parts.append(f'<div class="desc-text">{desc_text}</div>')
    if value_text:
        parts.append(
            f'<div class="desc-value"><strong>Значение:</strong> '
            f"{value_text}</div>"
        )
    if purpose_text:
        parts.append(
            f'<div class="desc-purpose">💡 {purpose_text}</div>'
        )
    if links_html:
        parts.append(f'<div class="desc-links">{links_html}</div>')

    return ("".join(parts), key)


def enrich_kv_rows(
    rows: list[tuple[str, str]],
) -> list[tuple[str, str, str, str]]:
    """Расширить [(label, value), ...] → [(label, value, desc_html, anchor), ...].

    desc_html — готовый HTML 3-й колонки (с cross-module links).
    anchor — id для <tr> (для cross-module deep-links).
    """
    out: list[tuple[str, str, str, str]] = []
    for label, value in rows:
        desc_html, anchor = render_description_html(label)
        out.append((label, value, desc_html, anchor))
    return out
