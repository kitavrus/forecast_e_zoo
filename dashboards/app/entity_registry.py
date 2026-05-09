"""Реестр сущностей дашборда — единый источник правды для страниц списка/карточки.

Каждая запись описывает таблицу БД (схема, имя, PK, отображаемые колонки,
колонки поиска и сортировку по умолчанию). Routes в `entity_routes.py`
читают этот реестр и строят SQL только из server-side whitelist — имена
из URL никогда не подставляются в SQL напрямую.
"""
from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class EntityDef:
    """Метаданные одной таблицы для генерации страниц списка и карточки."""

    schema: str
    table: str
    pk: tuple[str, ...]                          # композитный PK поддерживается
    list_columns: tuple[str, ...]                # что показываем в таблице списка
    search_columns: tuple[str, ...]              # колонки для ILIKE-поиска
    order_by: str                                # дефолтный ORDER BY (без префикса)
    friendly_name: str                           # русское имя для шапки
    large: bool = False                          # для больших таблиц use reltuples (без COUNT*)
    fk_links: dict[str, tuple[str, str]] = field(default_factory=dict)
    # fk_links: колонка → (схема, таблица) ссылочной сущности (PK предполагается single 'id')


# Полное имя таблицы → описание. Ключ — (schema, table).
ENTITIES: dict[tuple[str, str], EntityDef] = {
    # =========================================================================
    # public.* — M1 source-adapter (16 mock-erp entities + service tables)
    # =========================================================================
    ("public", "products"): EntityDef(
        schema="public", table="products",
        pk=("id",),
        list_columns=("id", "sku", "name", "category_id", "unit", "pack_size",
                      "is_active", "updated_at"),
        search_columns=("id", "sku", "name"),
        order_by="updated_at DESC NULLS LAST",
        friendly_name="Товары",
        fk_links={"category_id": ("public", "category")},
    ),
    ("public", "supplier"): EntityDef(
        schema="public", table="supplier",
        pk=("id",),
        list_columns=("id", "name", "inn", "kpp", "updated_at"),
        search_columns=("id", "name", "inn"),
        order_by="updated_at DESC NULLS LAST",
        friendly_name="Поставщики",
    ),
    ("public", "category"): EntityDef(
        schema="public", table="category",
        pk=("id",),
        list_columns=("id", "parent_id", "name", "updated_at"),
        search_columns=("id", "name"),
        order_by="name ASC",
        friendly_name="Категории",
        fk_links={"parent_id": ("public", "category")},
    ),
    ("public", "location"): EntityDef(
        schema="public", table="location",
        pk=("id",),
        list_columns=("id", "type", "name", "region", "address", "updated_at"),
        search_columns=("id", "name", "region", "address"),
        order_by="name ASC",
        friendly_name="Точки и склады",
    ),
    ("public", "product_barcodes"): EntityDef(
        schema="public", table="product_barcodes",
        pk=("product_id", "barcode"),
        list_columns=("product_id", "barcode", "is_primary"),
        search_columns=("product_id", "barcode"),
        order_by="product_id ASC",
        friendly_name="Штрихкоды товаров",
        fk_links={"product_id": ("public", "products")},
    ),
    ("public", "supply_spec"): EntityDef(
        schema="public", table="supply_spec",
        pk=("product_id", "supplier_id", "valid_from"),
        list_columns=("product_id", "supplier_id", "pack_qty", "lead_time_days",
                      "min_order_qty", "multiple", "valid_from", "valid_to"),
        search_columns=("product_id", "supplier_id"),
        order_by="valid_from DESC",
        friendly_name="Спецификации поставки",
        fk_links={"product_id": ("public", "products"),
                  "supplier_id": ("public", "supplier")},
    ),
    ("public", "promo"): EntityDef(
        schema="public", table="promo",
        pk=("id",),
        list_columns=("id", "location_id", "product_id", "start_date", "end_date",
                      "discount_pct", "updated_at"),
        search_columns=("id", "location_id", "product_id"),
        order_by="start_date DESC",
        friendly_name="Акции",
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products")},
    ),
    ("public", "order_rule"): EntityDef(
        schema="public", table="order_rule",
        pk=("id",),
        list_columns=("id", "location_id", "product_id", "category_id",
                      "rule_type", "valid_from", "valid_to"),
        search_columns=("id", "location_id", "product_id", "rule_type"),
        order_by="valid_from DESC",
        friendly_name="Правила заказа",
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products"),
                  "category_id": ("public", "category")},
    ),
    ("public", "supply_plan"): EntityDef(
        schema="public", table="supply_plan",
        pk=("id",),
        list_columns=("id", "location_id", "product_id", "supplier_id",
                      "plan_date", "qty"),
        search_columns=("id", "location_id", "product_id", "supplier_id"),
        order_by="plan_date DESC",
        friendly_name="План поставок",
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products"),
                  "supplier_id": ("public", "supplier")},
    ),
    ("public", "master_change_log"): EntityDef(
        schema="public", table="master_change_log",
        pk=("id",),
        list_columns=("id", "entity_type", "entity_id", "operation",
                      "changed_at"),
        search_columns=("entity_type", "entity_id", "operation"),
        order_by="changed_at DESC NULLS LAST",
        friendly_name="Журнал изменений мастеров",
    ),
    ("public", "store_assortment"): EntityDef(
        schema="public", table="store_assortment",
        pk=("location_id", "product_id"),
        list_columns=("location_id", "product_id", "start_date", "end_date",
                      "is_active", "updated_at"),
        search_columns=("location_id", "product_id"),
        order_by="updated_at DESC NULLS LAST",
        friendly_name="Ассортимент точек",
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products")},
    ),
    ("public", "store_assortment_lifecycle_events"): EntityDef(
        schema="public", table="store_assortment_lifecycle_events",
        pk=("id",),
        list_columns=("id", "location_id", "product_id", "event_type",
                      "event_date"),
        search_columns=("location_id", "product_id", "event_type"),
        order_by="event_date DESC",
        friendly_name="События lifecycle ассортимента",
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products")},
    ),
    ("public", "receipt_line"): EntityDef(
        schema="public", table="receipt_line",
        pk=("event_date", "id"),
        list_columns=("id", "receipt_id", "location_id", "product_id",
                      "qty", "price", "event_time", "event_date"),
        search_columns=("receipt_id", "location_id", "product_id"),
        order_by="event_time DESC NULLS LAST",
        friendly_name="Строки чеков",
        large=True,
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products")},
    ),
    ("public", "location_stock_snapshot"): EntityDef(
        schema="public", table="location_stock_snapshot",
        pk=("event_date", "location_id", "product_id"),
        list_columns=("event_date", "location_id", "product_id",
                      "qty_on_hand", "qty_reserved", "as_of"),
        search_columns=("location_id", "product_id"),
        order_by="event_date DESC, location_id ASC",
        friendly_name="Остатки на точках",
        large=True,
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products")},
    ),
    ("public", "stock_movement"): EntityDef(
        schema="public", table="stock_movement",
        pk=("event_date", "id"),
        list_columns=("id", "event_date", "event_time", "location_id",
                      "product_id", "movement_type", "qty", "ref_id"),
        search_columns=("location_id", "product_id", "movement_type", "ref_id"),
        order_by="event_time DESC NULLS LAST",
        friendly_name="Движения остатков",
        large=True,
        fk_links={"location_id": ("public", "location"),
                  "product_id": ("public", "products")},
    ),
    ("public", "supplier_stock_snapshot"): EntityDef(
        schema="public", table="supplier_stock_snapshot",
        pk=("event_date", "supplier_id", "product_id"),
        list_columns=("event_date", "supplier_id", "product_id",
                      "qty_available", "as_of"),
        search_columns=("supplier_id", "product_id"),
        order_by="event_date DESC, supplier_id ASC",
        friendly_name="Остатки у поставщиков",
        large=True,
        fk_links={"supplier_id": ("public", "supplier"),
                  "product_id": ("public", "products")},
    ),
    ("public", "loads"): EntityDef(
        schema="public", table="loads",
        pk=("load_id",),
        list_columns=("load_id", "status", "started_at", "committed_at",
                      "failed_at", "source", "lines_total", "lines_failed"),
        search_columns=("status", "source"),
        order_by="started_at DESC NULLS LAST",
        friendly_name="Загрузки M1",
    ),

    # =========================================================================
    # marts.* — M2 ETL validation
    # =========================================================================
    ("marts", "etl_runs"): EntityDef(
        schema="marts", table="etl_runs",
        pk=("id",),
        list_columns=("id", "status", "kind", "trigger", "target_mart",
                      "started_at", "finished_at", "committed_at",
                      "lines_total", "lines_failed"),
        search_columns=("status", "kind", "trigger", "target_mart", "requester"),
        order_by="started_at DESC NULLS LAST",
        friendly_name="ETL прогоны",
    ),
    ("marts", "reject_log"): EntityDef(
        schema="marts", table="reject_log",
        pk=("id",),
        list_columns=("id", "etl_run_id", "entity", "severity", "rule_id",
                      "field", "business_key", "created_at"),
        search_columns=("entity", "severity", "rule_id", "field", "business_key",
                        "message"),
        order_by="created_at DESC NULLS LAST",
        friendly_name="Журнал reject (ETL)",
        fk_links={"etl_run_id": ("marts", "etl_runs")},
    ),
    ("marts", "mart_master_current"): EntityDef(
        schema="marts", table="mart_master_current",
        pk=("entity_type", "entity_id"),
        list_columns=("entity_type", "entity_id", "etl_run_id",
                      "source_load_id", "created_at"),
        search_columns=("entity_type", "entity_id"),
        order_by="entity_type ASC, entity_id ASC",
        friendly_name="Mart: текущий снимок мастеров",
        fk_links={"etl_run_id": ("marts", "etl_runs")},
    ),
    ("marts", "mart_calculation_input"): EntityDef(
        schema="marts", table="mart_calculation_input",
        pk=("product_id", "location_id"),
        list_columns=("product_id", "location_id", "on_hand", "in_transit",
                      "safety_stock", "daily_demand", "supplier_id",
                      "lead_time_days"),
        search_columns=("product_id", "location_id", "supplier_id",
                        "applicable_rule_id", "applicable_rule_kind"),
        order_by="on_hand DESC NULLS LAST",
        friendly_name="Mart: входные данные калькулятора",
    ),
    ("marts", "mart_demand_history"): EntityDef(
        schema="marts", table="mart_demand_history",
        pk=("product_id", "location_id", "as_of_date"),
        list_columns=("as_of_date", "location_id", "product_id", "qty_sold",
                      "revenue_paid", "transactions_count", "had_promo",
                      "was_in_assortment"),
        search_columns=("product_id", "location_id"),
        order_by="qty_sold DESC NULLS LAST",
        friendly_name="Mart: история спроса",
        large=True,
    ),
    ("marts", "mart_kpi_daily"): EntityDef(
        schema="marts", table="mart_kpi_daily",
        pk=("location_id", "kpi_name", "as_of_date"),
        list_columns=("as_of_date", "location_id", "kpi_name", "kpi_value",
                      "kpi_unit"),
        search_columns=("location_id", "kpi_name"),
        order_by="as_of_date DESC, kpi_name ASC",
        friendly_name="Mart: KPI по дням",
        large=True,
    ),
    ("marts", "mart_supplier_scorecard"): EntityDef(
        schema="marts", table="mart_supplier_scorecard",
        pk=("supplier_id", "week_start"),
        list_columns=("supplier_id", "week_start", "fill_rate_avg", "otif_pct",
                      "lead_time_actual_avg", "qty_short_total",
                      "lines_delivered", "lines_late"),
        search_columns=("supplier_id",),
        order_by="week_start DESC NULLS LAST",
        friendly_name="Mart: scorecard поставщиков",
        fk_links={"supplier_id": ("public", "supplier")},
    ),

    # =========================================================================
    # kpi.* — M4
    # =========================================================================
    ("kpi", "kpi_snapshots"): EntityDef(
        schema="kpi", table="kpi_snapshots",
        pk=("id", "as_of_date"),
        list_columns=("as_of_date", "kpi_name", "scope_type", "scope_id",
                      "value", "computed_at"),
        search_columns=("kpi_name", "scope_type", "scope_id"),
        order_by="as_of_date DESC, kpi_name ASC",
        friendly_name="KPI snapshots",
        large=True,
    ),
    ("kpi", "kpi_calibrations"): EntityDef(
        schema="kpi", table="kpi_calibrations",
        pk=("id",),
        list_columns=("id", "kpi_name", "scope_type", "scope_id",
                      "created_at", "updated_at"),
        search_columns=("kpi_name", "scope_type", "scope_id"),
        order_by="updated_at DESC NULLS LAST",
        friendly_name="KPI калибровки",
    ),

    # =========================================================================
    # forecast.* — M5
    # =========================================================================
    ("forecast", "forecast_runs"): EntityDef(
        schema="forecast", table="forecast_runs",
        pk=("id",),
        list_columns=("id", "status", "horizon_days", "started_at",
                      "finished_at", "forecasts_count", "lines_count",
                      "plans_count"),
        search_columns=("status",),
        order_by="started_at DESC NULLS LAST",
        friendly_name="Прогон Forecast Engine",
    ),
    ("forecast", "forecasts"): EntityDef(
        schema="forecast", table="forecasts",
        pk=("run_id", "product_id", "location_id", "forecast_date"),
        list_columns=("run_id", "product_id", "location_id", "forecast_date",
                      "forecast_qty", "lower_bound", "upper_bound",
                      "model_name"),
        search_columns=("product_id", "location_id", "model_name"),
        order_by="forecast_date DESC, forecast_qty DESC NULLS LAST",
        friendly_name="Прогнозы",
        large=True,
        fk_links={"run_id": ("forecast", "forecast_runs")},
    ),
    ("forecast", "replenishment_plans"): EntityDef(
        schema="forecast", table="replenishment_plans",
        pk=("id",),
        list_columns=("id", "run_id", "supplier_id", "location_id",
                      "plan_date", "total_qty", "lines_count", "status",
                      "approved_at", "approved_by"),
        search_columns=("supplier_id", "location_id", "status", "approved_by"),
        order_by="plan_date DESC, supplier_id ASC",
        friendly_name="Планы пополнения",
        fk_links={"run_id": ("forecast", "forecast_runs"),
                  "supplier_id": ("public", "supplier"),
                  "location_id": ("public", "location")},
    ),

    # =========================================================================
    # orders.* — M6
    # =========================================================================
    ("orders", "purchase_orders"): EntityDef(
        schema="orders", table="purchase_orders",
        pk=("id", "created_at"),
        list_columns=("po_number", "supplier_id", "location_id", "status",
                      "total_qty", "total_amount", "currency", "delivery_date",
                      "created_at"),
        search_columns=("po_number", "supplier_id", "location_id", "status",
                        "sent_to_channel"),
        order_by="created_at DESC NULLS LAST",
        friendly_name="Purchase Orders",
        fk_links={"supplier_id": ("public", "supplier"),
                  "location_id": ("public", "location")},
    ),
    ("orders", "po_lines"): EntityDef(
        schema="orders", table="po_lines",
        pk=("id",),
        list_columns=("id", "po_id", "product_id", "qty", "unit_price",
                      "line_amount", "pricing_source", "created_at"),
        search_columns=("po_id", "product_id", "pricing_source"),
        order_by="created_at DESC NULLS LAST",
        friendly_name="Строки PO",
        fk_links={"product_id": ("public", "products")},
    ),

    # =========================================================================
    # channels.* — M7
    # =========================================================================
    ("channels", "send_attempts"): EntityDef(
        schema="channels", table="send_attempts",
        pk=("id", "started_at"),
        list_columns=("id", "po_id", "supplier_id", "channel_type", "status",
                      "http_status_code", "started_at", "finished_at",
                      "retry_count", "external_ref"),
        search_columns=("po_id", "supplier_id", "channel_type", "status",
                        "external_ref"),
        order_by="started_at DESC NULLS LAST",
        friendly_name="Попытки отправки в канал",
        fk_links={"supplier_id": ("public", "supplier")},
    ),
    ("channels", "supplier_channel_config"): EntityDef(
        schema="channels", table="supplier_channel_config",
        pk=("supplier_id",),
        list_columns=("supplier_id", "channel_type", "endpoint_url",
                      "auth_mode", "timeout_sec", "retry_max", "is_active",
                      "updated_at"),
        search_columns=("supplier_id", "channel_type", "endpoint_url",
                        "auth_mode"),
        order_by="supplier_id ASC",
        friendly_name="Конфиг каналов поставщиков",
        fk_links={"supplier_id": ("public", "supplier")},
    ),
}


def get_entity(schema: str, table: str) -> EntityDef | None:
    """Lookup сущности по (schema, table). None если не найдена в whitelist."""
    return ENTITIES.get((schema, table))


def list_entities() -> list[EntityDef]:
    """Все сущности — для индекса/обзорной страницы (если понадобится)."""
    return list(ENTITIES.values())
