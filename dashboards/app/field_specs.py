"""Field-level specifications for entities surfaced on each /m{N} dashboard page.

Sources of truth:
- mock-erp/app/models.py (SQLModel)
- internal/features/data_export/sqls/migrations/0001-0002 (public schema)
- internal/features/etl_validation/sqls/migrations/1001 (marts schema)
- internal/features/kpi/sqls/migrations/2001 (kpi schema)
- internal/features/forecast/sqls/migrations/3001 (forecast schema)
- internal/features/orders/sqls/migrations/4001 (orders schema)
- internal/features/channels/sqls/migrations/5001 (channels schema)

Каждая EntitySpec содержит список полей с типом, диапазоном/форматом, примером
и кратким назначением — так пользователь видит контракт данных, а не только
название таблицы.
"""
from __future__ import annotations

from typing import TypedDict


class FieldSpec(TypedDict):
    name: str
    type: str
    range: str
    example: str
    purpose: str


class EntitySpec(TypedDict):
    name: str
    title: str
    description: str
    fields: list[FieldSpec]


# --- Helpers (для DRY: reusable common fields) -------------------------------

_F_ETL_RUN_ID: FieldSpec = {
    "name": "etl_run_id",
    "type": "UUID",
    "range": "RFC 4122",
    "example": "9f8e7d6c-1a2b-3c4d-5e6f-7a8b9c0d1e2f",
    "purpose": "ID запуска ETL, в котором появилась строка (для traceability)",
}
_F_SOURCE_LOAD_ID: FieldSpec = {
    "name": "source_load_id",
    "type": "UUID",
    "range": "RFC 4122",
    "example": "11111111-2222-3333-4444-555555555555",
    "purpose": "ID load-снапшота из public.snapshot_pointer (M1)",
}
_F_CREATED_AT: FieldSpec = {
    "name": "created_at",
    "type": "timestamptz",
    "range": "ISO-8601 UTC",
    "example": "2026-05-08T10:30:00Z",
    "purpose": "Время вставки строки в БД",
}
_F_UPDATED_AT: FieldSpec = {
    "name": "updated_at",
    "type": "timestamptz",
    "range": "ISO-8601 UTC",
    "example": "2026-05-08T10:30:00Z",
    "purpose": "Время последнего обновления строки",
}


# --- m0 / m1 (mock-erp + public.* mirror) ------------------------------------

# 16 entities mock-erp = 16 public.* таблиц в M1 (имя совпадает 1:1).

_PRODUCTS: EntitySpec = {
    "name": "products",
    "title": "products — каталог товаров",
    "description": "Master entity. Каталог товарных позиций E-Zoo (~корм, аксессуары, ветеринария).",
    "fields": [
        {"name": "id", "type": "string (TEXT)", "range": "UUID или ERP-ID", "example": "P-00042", "purpose": "Первичный ключ товара в системе ERP"},
        {"name": "sku", "type": "string", "range": "латиница/цифры/дефис, 3–50 chars", "example": "RC-MINI-800", "purpose": "Артикул товара, видимый клиенту"},
        {"name": "name", "type": "string", "range": "UTF-8, 1–200 chars", "example": "Royal Canin Mini Adult 800g", "purpose": "Название товара для отображения"},
        {"name": "category_id", "type": "string", "range": "FK → category.id", "example": "C-FOOD-DOG", "purpose": "Принадлежность к категории каталога"},
        {"name": "unit", "type": "string", "range": "'pcs' | 'kg' | 'l' | 'pack'", "example": "pcs", "purpose": "Единица измерения (для расчёта qty)"},
        {"name": "pack_size", "type": "float", "range": "> 0", "example": "12", "purpose": "Размер упаковки в единицах unit"},
        {"name": "is_active", "type": "bool", "range": "true/false", "example": "true", "purpose": "Активен ли товар в каталоге сейчас"},
        {"name": "attributes", "type": "JSONB", "range": "произвольный JSON", "example": '{"weight_g":800,"breed":"mini"}', "purpose": "Дополнительные характеристики товара"},
        _F_UPDATED_AT,
    ],
}

_PRODUCT_BARCODES: EntitySpec = {
    "name": "product_barcodes",
    "title": "product_barcodes — штрихкоды",
    "description": "1 товар → N штрихкодов (EAN/UPC). PK = barcode, FK product_id.",
    "fields": [
        {"name": "barcode", "type": "string", "range": "EAN-13/UPC, 8–14 цифр", "example": "4607034567893", "purpose": "Штрихкод (PK)"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Какому товару принадлежит штрихкод"},
    ],
}

_CATEGORY: EntitySpec = {
    "name": "category",
    "title": "category — категории каталога",
    "description": "Иерархический справочник (path = 'food/dog/dry').",
    "fields": [
        {"name": "id", "type": "string", "range": "C-XXX", "example": "C-FOOD-DOG", "purpose": "Первичный ключ категории"},
        {"name": "name", "type": "string", "range": "UTF-8, 1–100 chars", "example": "Корм для собак", "purpose": "Название категории"},
        {"name": "path", "type": "string", "range": "slash-separated", "example": "food/dog/dry", "purpose": "Полный путь в дереве категорий"},
        _F_UPDATED_AT,
    ],
}

_LOCATION: EntitySpec = {
    "name": "location",
    "title": "location — точки продаж и склады",
    "description": "Магазины, ДЦ, dark-stores. type определяет роль в supply chain.",
    "fields": [
        {"name": "id", "type": "string", "range": "L-NNN", "example": "L-001", "purpose": "Первичный ключ локации"},
        {"name": "type", "type": "string", "range": "'store' | 'dc' | 'dark_store'", "example": "store", "purpose": "Тип локации (магазин / распред. центр)"},
        {"name": "name", "type": "string", "range": "UTF-8", "example": "E-Zoo Киев Печерск", "purpose": "Название точки"},
        {"name": "region", "type": "string", "range": "UTF-8", "example": "Киев", "purpose": "Регион для группировки KPI"},
        _F_UPDATED_AT,
    ],
}

_SUPPLIER: EntitySpec = {
    "name": "supplier",
    "title": "supplier — поставщики",
    "description": "Контрагенты-поставщики; используются в supply_spec и replenishment_plans.",
    "fields": [
        {"name": "id", "type": "string", "range": "S-NNN", "example": "S-042", "purpose": "Первичный ключ поставщика"},
        {"name": "name", "type": "string", "range": "UTF-8", "example": "ТОВ Royal Canin Україна", "purpose": "Название контрагента"},
        {"name": "inn", "type": "string | null", "range": "10/12 цифр", "example": "1234567890", "purpose": "Налоговый номер (опционально)"},
        _F_UPDATED_AT,
    ],
}

_SUPPLY_SPEC: EntitySpec = {
    "name": "supply_spec",
    "title": "supply_spec — условия поставки",
    "description": "Правила поставки per (product, supplier, valid_from): кратность, MOQ, lead time.",
    "fields": [
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Товар"},
        {"name": "supplier_id", "type": "string", "range": "FK → supplier.id", "example": "S-042", "purpose": "Поставщик"},
        {"name": "pack_qty", "type": "int", "range": "≥ 1", "example": "12", "purpose": "Количество единиц в палете/коробке"},
        {"name": "lead_time_days", "type": "int", "range": "1–60", "example": "7", "purpose": "Срок поставки в днях (для расчёта reorder point)"},
        {"name": "min_order_qty", "type": "int", "range": "≥ 1", "example": "24", "purpose": "MOQ — минимальный заказ"},
        {"name": "multiple", "type": "int", "range": "≥ 1", "example": "12", "purpose": "Кратность заказа"},
        {"name": "valid_from", "type": "timestamptz", "range": "ISO-8601", "example": "2026-01-01T00:00:00Z", "purpose": "С какой даты действуют условия"},
    ],
}

_PROMO: EntitySpec = {
    "name": "promo",
    "title": "promo — акции и скидки",
    "description": "Промо per (location, product, period). discount_pct учитывается в demand history.",
    "fields": [
        {"name": "id", "type": "string", "range": "PR-NNN", "example": "PR-2026-05-01", "purpose": "ID промо-кампании"},
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "Где действует акция"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "На какой товар"},
        {"name": "start_date", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-01T00:00:00Z", "purpose": "Начало акции"},
        {"name": "end_date", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-15T23:59:59Z", "purpose": "Конец акции"},
        {"name": "discount_pct", "type": "int", "range": "0–100", "example": "20", "purpose": "Размер скидки в процентах"},
        _F_UPDATED_AT,
    ],
}

_ORDER_RULE: EntitySpec = {
    "name": "order_rule",
    "title": "order_rule — правила заказа",
    "description": "Бизнес-правила per location. payload (JSON) задаёт параметры конкретного rule_type.",
    "fields": [
        {"name": "id", "type": "string", "range": "OR-NNN", "example": "OR-001", "purpose": "ID правила"},
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "К какой локации применяется"},
        {"name": "rule_type", "type": "string", "range": "'min_max' | 'reorder_point' | 'fixed_qty'", "example": "reorder_point", "purpose": "Тип правила"},
        {"name": "payload", "type": "JSONB", "range": "зависит от rule_type", "example": '{"min":10,"max":50}', "purpose": "Параметры правила"},
        {"name": "valid_from", "type": "timestamptz", "range": "ISO-8601", "example": "2026-01-01T00:00:00Z", "purpose": "С какой даты действует"},
    ],
}

_SUPPLY_PLAN: EntitySpec = {
    "name": "supply_plan",
    "title": "supply_plan — план поставок",
    "description": "Уже согласованные с поставщиком поставки (in_transit для расчёта on_hand+in_transit).",
    "fields": [
        {"name": "id", "type": "string", "range": "SP-NNN", "example": "SP-2026-05-08", "purpose": "ID плановой поставки"},
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "Куда поставка"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Что поставляется"},
        {"name": "supplier_id", "type": "string", "range": "FK → supplier.id", "example": "S-042", "purpose": "Кто поставляет"},
        {"name": "plan_date", "type": "timestamptz", "range": "ISO-8601 (date)", "example": "2026-05-15T00:00:00Z", "purpose": "Дата ожидаемой поставки"},
        {"name": "qty", "type": "int", "range": "> 0", "example": "120", "purpose": "Объём поставки"},
    ],
}

_STORE_ASSORTMENT: EntitySpec = {
    "name": "store_assortment",
    "title": "store_assortment — ассортимент магазина",
    "description": "Какие товары присутствуют в каждой локации (slowly changing).",
    "fields": [
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "Локация"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Товар"},
        {"name": "start_date", "type": "timestamptz", "range": "ISO-8601", "example": "2026-01-01T00:00:00Z", "purpose": "С какой даты товар в ассортименте"},
        {"name": "is_active", "type": "bool", "range": "true/false", "example": "true", "purpose": "Активен ли в ассортименте"},
        _F_UPDATED_AT,
    ],
}

_STORE_ASSORTMENT_LIFECYCLE: EntitySpec = {
    "name": "store_assortment_lifecycle_events",
    "title": "store_assortment_lifecycle_events — события жизненного цикла",
    "description": "Audit log изменений ассортимента (add/remove/promo_only).",
    "fields": [
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "Локация"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Товар"},
        {"name": "event_type", "type": "string", "range": "'added' | 'removed' | 'promo_only'", "example": "added", "purpose": "Что произошло"},
        {"name": "event_date", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T10:00:00Z", "purpose": "Когда событие случилось"},
        {"name": "payload", "type": "JSONB", "range": "произвольный JSON", "example": '{"reason":"new_supplier"}', "purpose": "Дополнительный контекст"},
    ],
}

_MASTER_CHANGE_LOG: EntitySpec = {
    "name": "master_change_log",
    "title": "master_change_log — лог изменений мастер-данных",
    "description": "Audit любых изменений master сущностей (products/category/...).",
    "fields": [
        {"name": "entity", "type": "string", "range": "'products' | 'category' | ...", "example": "products", "purpose": "Какая сущность изменилась"},
        {"name": "entity_pk", "type": "JSONB", "range": "{key: value}", "example": '{"id":"P-00042"}', "purpose": "PK изменённой строки"},
        {"name": "field", "type": "string", "range": "имя колонки", "example": "name", "purpose": "Какое поле изменилось"},
        {"name": "old_value", "type": "string | null", "range": "stringified", "example": "Royal Canin Mini 500g", "purpose": "Старое значение"},
        {"name": "new_value", "type": "string | null", "range": "stringified", "example": "Royal Canin Mini Adult 800g", "purpose": "Новое значение"},
        {"name": "changed_at", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T10:30:00Z", "purpose": "Когда поменялось"},
    ],
}

_RECEIPT_LINE: EntitySpec = {
    "name": "receipt_line",
    "title": "receipt_line — строки чеков продаж",
    "description": "Fact-таблица: каждая строка = один товар в чеке. Партиционирована monthly по event_date.",
    "fields": [
        {"name": "id", "type": "bigint", "range": "auto-increment", "example": "12345", "purpose": "PK строки чека"},
        {"name": "receipt_id", "type": "string", "range": "R-YYYYMMDD-NNNN", "example": "R-20260508-0042", "purpose": "ID чека (несколько строк один чек)"},
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "Где произошла продажа"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Что продали"},
        {"name": "qty", "type": "int", "range": "может быть < 0 при возврате", "example": "2", "purpose": "Количество (отрицательное = возврат)"},
        {"name": "price", "type": "float", "range": "≥ 0", "example": "289.50", "purpose": "Цена за единицу на момент продажи"},
        {"name": "event_time", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T14:23:11Z", "purpose": "Точное время продажи"},
        {"name": "event_date", "type": "timestamptz", "range": "ISO-8601 (date)", "example": "2026-05-08T00:00:00Z", "purpose": "Ключ партиционирования (день)"},
        {"name": "payload", "type": "JSONB", "range": "произвольный JSON", "example": '{"promo_id":"PR-2026-05-01","cashier":"K42"}', "purpose": "Доп. контекст продажи"},
    ],
}

_LOCATION_STOCK_SNAPSHOT: EntitySpec = {
    "name": "location_stock_snapshot",
    "title": "location_stock_snapshot — остатки по локациям",
    "description": "Дневные снимки остатков (on_hand + reserved). Партиционирована по event_date.",
    "fields": [
        {"name": "event_date", "type": "timestamptz", "range": "ISO-8601 (date)", "example": "2026-05-08T00:00:00Z", "purpose": "Дата снимка (партиционный ключ)"},
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "Локация"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Товар"},
        {"name": "qty_on_hand", "type": "int", "range": "≥ 0", "example": "47", "purpose": "Доступный остаток"},
        {"name": "qty_reserved", "type": "int", "range": "≥ 0", "example": "5", "purpose": "Зарезервировано (онлайн-заказы и т.п.)"},
        {"name": "as_of", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T03:00:00Z", "purpose": "Точное время снимка"},
    ],
}

_STOCK_MOVEMENT: EntitySpec = {
    "name": "stock_movement",
    "title": "stock_movement — движения остатков",
    "description": "Каждое перемещение/списание/приход. Партиционирована по event_date.",
    "fields": [
        {"name": "id", "type": "bigint", "range": "auto-increment", "example": "98765", "purpose": "PK движения"},
        {"name": "event_date", "type": "timestamptz", "range": "ISO-8601 (date)", "example": "2026-05-08T00:00:00Z", "purpose": "Дата движения (партиционный ключ)"},
        {"name": "event_time", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T14:23:11Z", "purpose": "Точное время движения"},
        {"name": "location_id", "type": "string", "range": "FK → location.id", "example": "L-001", "purpose": "Где произошло движение"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Товар"},
        {"name": "movement_type", "type": "string", "range": "'sale' | 'return' | 'transfer_in' | 'transfer_out' | 'write_off' | 'receipt'", "example": "sale", "purpose": "Тип движения"},
        {"name": "qty", "type": "int", "range": "может быть отрицательным", "example": "-2", "purpose": "Изменение остатка (sign по типу)"},
        {"name": "ref_id", "type": "string", "range": "ID связанного документа", "example": "R-20260508-0042", "purpose": "Ссылка на источник (чек/перемещение)"},
        {"name": "payload", "type": "JSONB", "range": "произвольный JSON", "example": '{"reason":"expired"}', "purpose": "Доп. контекст"},
    ],
}

_SUPPLIER_STOCK_SNAPSHOT: EntitySpec = {
    "name": "supplier_stock_snapshot",
    "title": "supplier_stock_snapshot — остатки на стороне поставщика",
    "description": "Дневные снимки доступности у поставщика (для расчёта риска OOS).",
    "fields": [
        {"name": "event_date", "type": "timestamptz", "range": "ISO-8601 (date)", "example": "2026-05-08T00:00:00Z", "purpose": "Дата снимка"},
        {"name": "supplier_id", "type": "string", "range": "FK → supplier.id", "example": "S-042", "purpose": "Поставщик"},
        {"name": "product_id", "type": "string", "range": "FK → products.id", "example": "P-00042", "purpose": "Товар"},
        {"name": "qty", "type": "int", "range": "≥ 0", "example": "5000", "purpose": "Сколько доступно у поставщика"},
        {"name": "as_of", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T03:00:00Z", "purpose": "Время снимка"},
    ],
}

ALL_ERP_ENTITIES: list[EntitySpec] = [
    _PRODUCTS,
    _PRODUCT_BARCODES,
    _CATEGORY,
    _LOCATION,
    _SUPPLIER,
    _SUPPLY_SPEC,
    _PROMO,
    _ORDER_RULE,
    _SUPPLY_PLAN,
    _STORE_ASSORTMENT,
    _STORE_ASSORTMENT_LIFECYCLE,
    _MASTER_CHANGE_LOG,
    _RECEIPT_LINE,
    _LOCATION_STOCK_SNAPSHOT,
    _STOCK_MOVEMENT,
    _SUPPLIER_STOCK_SNAPSHOT,
]


# --- m2 / m3 (marts schema) --------------------------------------------------

_MART_MASTER_CURRENT: EntitySpec = {
    "name": "marts.mart_master_current",
    "title": "mart_master_current — текущий снимок мастер-данных",
    "description": "TRUNCATE+INSERT в одной транзакции. payload (JSONB) хранит полную сущность.",
    "fields": [
        {"name": "entity_type", "type": "string", "range": "'products' | 'category' | 'supplier' | ...", "example": "products", "purpose": "Тип сущности"},
        {"name": "entity_id", "type": "string", "range": "ID из source", "example": "P-00042", "purpose": "PK сущности (составной с entity_type)"},
        {"name": "payload", "type": "JSONB", "range": "вся сущность как JSON", "example": '{"id":"P-00042","sku":"RC-MINI-800","name":"Royal Canin..."}', "purpose": "Полное тело master entity"},
        _F_ETL_RUN_ID,
        _F_SOURCE_LOAD_ID,
        _F_CREATED_AT,
    ],
}

_MART_CALCULATION_INPUT: EntitySpec = {
    "name": "marts.mart_calculation_input",
    "title": "mart_calculation_input — вход для расчёта заказа",
    "description": "Текущий снимок (TRUNCATE+INSERT). Один из главных входов M5 (Forecast Engine).",
    "fields": [
        {"name": "product_id", "type": "string", "range": "FK products", "example": "P-00042", "purpose": "Товар"},
        {"name": "location_id", "type": "string", "range": "FK location", "example": "L-001", "purpose": "Локация"},
        {"name": "on_hand", "type": "numeric(18,4)", "range": "≥ 0", "example": "47.0000", "purpose": "Текущий остаток"},
        {"name": "in_transit", "type": "numeric(18,4)", "range": "≥ 0", "example": "120.0000", "purpose": "В пути по supply_plan"},
        {"name": "safety_stock", "type": "numeric(18,4) | null", "range": "≥ 0", "example": "30.0000", "purpose": "Страховой запас"},
        {"name": "forecast_horizon_days", "type": "int | null", "range": "1–90", "example": "14", "purpose": "Горизонт прогноза для этой пары"},
        {"name": "daily_demand", "type": "numeric(18,4) | null", "range": "≥ 0", "example": "4.5000", "purpose": "Средний расход в сутки"},
        {"name": "min_qty", "type": "numeric(18,4) | null", "range": "≥ 0", "example": "10.0000", "purpose": "Min из order_rule (если применимо)"},
        {"name": "max_qty", "type": "numeric(18,4) | null", "range": "≥ min_qty", "example": "60.0000", "purpose": "Max из order_rule"},
        {"name": "applicable_rule_id", "type": "string | null", "range": "FK order_rule.id", "example": "OR-001", "purpose": "Какое правило применилось"},
        {"name": "applicable_rule_kind", "type": "string", "range": "'min_max' | 'reorder_point' | 'fixed_qty' | 'default'", "example": "reorder_point", "purpose": "Тип правила"},
        _F_ETL_RUN_ID,
        _F_SOURCE_LOAD_ID,
    ],
}

_MART_DEMAND_HISTORY: EntitySpec = {
    "name": "marts.mart_demand_history",
    "title": "mart_demand_history — история спроса",
    "description": "Партиционирована monthly по as_of_date. Агрегаты per (product, location, day).",
    "fields": [
        {"name": "as_of_date", "type": "date", "range": "YYYY-MM-DD", "example": "2026-05-08", "purpose": "Дата (партиционный ключ)"},
        {"name": "location_id", "type": "string", "range": "FK location", "example": "L-001", "purpose": "Локация"},
        {"name": "product_id", "type": "string", "range": "FK products", "example": "P-00042", "purpose": "Товар"},
        {"name": "qty_sold", "type": "numeric(18,4)", "range": "≥ 0", "example": "5.0000", "purpose": "Продано за день"},
        {"name": "qty_returned", "type": "numeric(18,4)", "range": "≥ 0", "example": "0.0000", "purpose": "Возвращено за день"},
        {"name": "qty_promo_bonus", "type": "numeric(18,4)", "range": "≥ 0", "example": "1.0000", "purpose": "Бонусные единицы по акции"},
        {"name": "qty_gift", "type": "numeric(18,4)", "range": "≥ 0", "example": "0.0000", "purpose": "Подарочные единицы"},
        {"name": "revenue_paid", "type": "numeric(18,4)", "range": "≥ 0", "example": "1447.50", "purpose": "Выручка за день"},
        {"name": "discount_total", "type": "numeric(18,4)", "range": "≥ 0", "example": "289.50", "purpose": "Скидка в денежном выражении"},
        {"name": "transactions_count", "type": "int", "range": "≥ 0", "example": "3", "purpose": "Кол-во чеков, в которых был товар"},
        {"name": "had_promo", "type": "bool", "range": "true/false", "example": "true", "purpose": "Действовала ли акция в этот день"},
        {"name": "promo_type", "type": "string | null", "range": "'discount' | 'bundle' | 'gift'", "example": "discount", "purpose": "Тип акции"},
        {"name": "was_in_assortment", "type": "bool", "range": "true/false", "example": "true", "purpose": "Был ли в ассортименте"},
        {"name": "lifecycle_state_at_date", "type": "string | null", "range": "'active' | 'introduced' | 'phasing_out'", "example": "active", "purpose": "Стадия жизненного цикла"},
        {"name": "was_oos", "type": "bool", "range": "true/false", "example": "false", "purpose": "Был ли OOS (out of stock)"},
        _F_ETL_RUN_ID,
        _F_SOURCE_LOAD_ID,
    ],
}

_MART_KPI_DAILY: EntitySpec = {
    "name": "marts.mart_kpi_daily",
    "title": "mart_kpi_daily — daily KPI per location",
    "description": "Партиционирована monthly по as_of_date. PK (location, kpi_name, date).",
    "fields": [
        {"name": "as_of_date", "type": "date", "range": "YYYY-MM-DD", "example": "2026-05-08", "purpose": "Дата KPI"},
        {"name": "location_id", "type": "string", "range": "FK location", "example": "L-001", "purpose": "Локация"},
        {"name": "kpi_name", "type": "string", "range": "'osa' | 'otif' | 'stock_days' | ...", "example": "osa", "purpose": "Имя KPI"},
        {"name": "kpi_value", "type": "numeric(18,6)", "range": "по типу KPI", "example": "0.967500", "purpose": "Значение KPI"},
        {"name": "kpi_unit", "type": "string | null", "range": "'pct' | 'days' | 'count'", "example": "pct", "purpose": "Единица измерения"},
        _F_ETL_RUN_ID,
        _F_SOURCE_LOAD_ID,
    ],
}

_MART_SUPPLIER_SCORECARD: EntitySpec = {
    "name": "marts.mart_supplier_scorecard",
    "title": "mart_supplier_scorecard — еженедельная оценка поставщиков",
    "description": "PK (supplier_id, week_start). Используется в KPI и при выборе поставщика для PO.",
    "fields": [
        {"name": "supplier_id", "type": "string", "range": "FK supplier", "example": "S-042", "purpose": "Поставщик"},
        {"name": "week_start", "type": "date", "range": "понедельник недели", "example": "2026-05-04", "purpose": "Начало недели"},
        {"name": "fill_rate_avg", "type": "numeric(8,4) | null", "range": "0.0000–1.0000", "example": "0.9650", "purpose": "Средний fill rate"},
        {"name": "otif_pct", "type": "numeric(8,4) | null", "range": "0.0000–1.0000", "example": "0.9200", "purpose": "On-Time-In-Full доля"},
        {"name": "lead_time_actual_avg", "type": "numeric(10,2) | null", "range": "≥ 0", "example": "8.50", "purpose": "Фактический средний lead time"},
        {"name": "qty_short_total", "type": "numeric(18,4)", "range": "≥ 0", "example": "12.0000", "purpose": "Сумма недопоставок за неделю"},
        {"name": "qty_damaged_total", "type": "numeric(18,4)", "range": "≥ 0", "example": "3.0000", "purpose": "Сумма повреждённых единиц"},
        {"name": "qty_returned_total", "type": "numeric(18,4)", "range": "≥ 0", "example": "0.0000", "purpose": "Сумма возвратов"},
        {"name": "lines_delivered", "type": "int", "range": "≥ 0", "example": "47", "purpose": "Кол-во доставленных строк"},
        {"name": "lines_late", "type": "int", "range": "≥ 0", "example": "3", "purpose": "Кол-во поздних строк"},
        _F_ETL_RUN_ID,
        _F_SOURCE_LOAD_ID,
    ],
}

ALL_MART_ENTITIES: list[EntitySpec] = [
    _MART_MASTER_CURRENT,
    _MART_CALCULATION_INPUT,
    _MART_DEMAND_HISTORY,
    _MART_KPI_DAILY,
    _MART_SUPPLIER_SCORECARD,
]


# --- m4 (kpi schema) ---------------------------------------------------------

_KPI_CALIBRATIONS: EntitySpec = {
    "name": "kpi.kpi_calibrations",
    "title": "kpi_calibrations — иерархия параметров KPI",
    "description": "scope_type определяет уровень: global → category → supplier → location → product_location.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "11111111-2222-3333-4444-555555555555", "purpose": "PK"},
        {"name": "kpi_name", "type": "string", "range": "'osa' | 'otif' | 'stock_days'", "example": "osa", "purpose": "Имя KPI"},
        {"name": "scope_type", "type": "string", "range": "'global' | 'category' | 'supplier' | 'location' | 'product_location'", "example": "global", "purpose": "Уровень иерархии"},
        {"name": "scope_id", "type": "string | null", "range": "ID для не-global", "example": "L-001", "purpose": "Конкретная локация/категория (NULL для global)"},
        {"name": "params", "type": "JSONB", "range": "зависит от kpi_name", "example": '{"lookback_days":30,"min_observations":7}', "purpose": "Параметры расчёта"},
        _F_CREATED_AT,
        _F_UPDATED_AT,
    ],
}

_KPI_SNAPSHOTS: EntitySpec = {
    "name": "kpi.kpi_snapshots",
    "title": "kpi_snapshots — рассчитанные значения KPI",
    "description": "Партиционирована monthly по as_of_date. value — итог расчёта по calibration.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "purpose": "PK снапшота"},
        {"name": "as_of_date", "type": "date", "range": "YYYY-MM-DD", "example": "2026-05-08", "purpose": "На какую дату рассчитан KPI"},
        {"name": "kpi_name", "type": "string", "range": "'osa' | 'otif' | 'stock_days'", "example": "osa", "purpose": "Имя KPI"},
        {"name": "scope_type", "type": "string", "range": "'global' | 'category' | 'supplier' | 'location' | 'product_location'", "example": "location", "purpose": "Уровень scope"},
        {"name": "scope_id", "type": "string | null", "range": "ID или NULL", "example": "L-001", "purpose": "Конкретная сущность scope"},
        {"name": "value", "type": "numeric(18,6)", "range": "по семантике KPI", "example": "0.967500", "purpose": "Рассчитанное значение"},
        {"name": "calibration_id", "type": "UUID | null", "range": "FK kpi_calibrations.id", "example": "11111111-2222-3333-4444-555555555555", "purpose": "Какая calibration использовалась"},
        {"name": "computed_at", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T03:15:00Z", "purpose": "Когда был рассчитан"},
        _F_ETL_RUN_ID,
        _F_CREATED_AT,
    ],
}

ALL_KPI_ENTITIES: list[EntitySpec] = [_KPI_CALIBRATIONS, _KPI_SNAPSHOTS]


# --- m5 (forecast schema) ----------------------------------------------------

_FORECAST_RUNS: EntitySpec = {
    "name": "forecast.forecast_runs",
    "title": "forecast_runs — реестр запусков движка прогнозирования",
    "description": "Один прогон M5 = одна строка. Все forecasts/calculation_lines/plans ссылаются на run_id.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "9f8e7d6c-1a2b-3c4d-5e6f-7a8b9c0d1e2f", "purpose": "PK прогона"},
        {"name": "started_at", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T04:00:00Z", "purpose": "Время старта"},
        {"name": "finished_at", "type": "timestamptz | null", "range": "ISO-8601", "example": "2026-05-08T04:03:42Z", "purpose": "Время финиша (NULL если running)"},
        {"name": "status", "type": "string", "range": "'running' | 'committed' | 'failed'", "example": "committed", "purpose": "Статус прогона"},
        {"name": "horizon_days", "type": "int", "range": "1–90, default 14", "example": "14", "purpose": "Горизонт прогноза"},
        {"name": "snapshot_etl_run_id", "type": "UUID | null", "range": "FK на mart etl_run_id", "example": "aaaa-bbbb-cccc-dddd-eeee", "purpose": "Какой mart-snapshot использовался"},
        {"name": "forecasts_count", "type": "int", "range": "≥ 0", "example": "12500", "purpose": "Сколько строк forecasts создано"},
        {"name": "lines_count", "type": "int", "range": "≥ 0", "example": "893", "purpose": "Сколько calculation_lines создано"},
        {"name": "plans_count", "type": "int", "range": "≥ 0", "example": "47", "purpose": "Сколько replenishment_plans создано"},
        {"name": "error_message", "type": "string | null", "range": "если status=failed", "example": "model timeout", "purpose": "Описание ошибки"},
        _F_CREATED_AT,
    ],
}

_FORECASTS: EntitySpec = {
    "name": "forecast.forecasts",
    "title": "forecasts — прогноз спроса по дням",
    "description": "Партиционирована monthly по forecast_date. PK (run_id, product_id, location_id, forecast_date).",
    "fields": [
        {"name": "run_id", "type": "UUID", "range": "FK forecast_runs.id", "example": "9f8e7d6c-...", "purpose": "К какому прогону относится"},
        {"name": "product_id", "type": "string", "range": "FK products", "example": "P-00042", "purpose": "Товар"},
        {"name": "location_id", "type": "string", "range": "FK location", "example": "L-001", "purpose": "Локация"},
        {"name": "forecast_date", "type": "date", "range": "YYYY-MM-DD", "example": "2026-05-15", "purpose": "Дата, на которую прогноз (партиционный ключ)"},
        {"name": "forecast_qty", "type": "numeric(18,4)", "range": "≥ 0", "example": "4.5000", "purpose": "Прогнозируемое количество"},
        {"name": "lower_bound", "type": "numeric(18,4) | null", "range": "≥ 0, ≤ forecast_qty", "example": "3.2000", "purpose": "Нижняя граница доверительного интервала"},
        {"name": "upper_bound", "type": "numeric(18,4) | null", "range": "≥ forecast_qty", "example": "6.1000", "purpose": "Верхняя граница ДИ"},
        {"name": "model_name", "type": "string", "range": "'sma_seasonal' | 'ewma' | ...", "example": "sma_seasonal", "purpose": "Какая модель использовалась"},
        {"name": "confidence", "type": "numeric(5,4) | null", "range": "0.0000–1.0000", "example": "0.8500", "purpose": "Уверенность модели"},
        _F_CREATED_AT,
    ],
}

_CALCULATION_LINES: EntitySpec = {
    "name": "forecast.calculation_lines",
    "title": "calculation_lines — точки заказа per (product, location)",
    "description": "Промежуточный артефакт: до агрегации в replenishment_plans. UNIQUE (run_id, product, location).",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "ffffffff-1111-2222-3333-444444444444", "purpose": "PK строки"},
        {"name": "run_id", "type": "UUID", "range": "FK forecast_runs", "example": "9f8e7d6c-...", "purpose": "К какому прогону"},
        {"name": "product_id", "type": "string", "range": "FK products", "example": "P-00042", "purpose": "Товар"},
        {"name": "location_id", "type": "string", "range": "FK location", "example": "L-001", "purpose": "Локация"},
        {"name": "supplier_id", "type": "string | null", "range": "FK supplier", "example": "S-042", "purpose": "Кто будет поставщиком (выбран по scorecard)"},
        {"name": "current_stock", "type": "numeric(18,4)", "range": "≥ 0", "example": "47.0000", "purpose": "Текущий on_hand"},
        {"name": "in_transit", "type": "numeric(18,4)", "range": "≥ 0", "example": "120.0000", "purpose": "В пути по supply_plan"},
        {"name": "daily_demand", "type": "numeric(18,4)", "range": "≥ 0", "example": "4.5000", "purpose": "Расход в сутки (из mart)"},
        {"name": "lead_time_days", "type": "int", "range": "1–60, default 7", "example": "7", "purpose": "Срок поставки"},
        {"name": "safety_stock", "type": "numeric(18,4)", "range": "≥ 0", "example": "30.0000", "purpose": "Страховой запас"},
        {"name": "reorder_point", "type": "numeric(18,4)", "range": "≥ safety_stock", "example": "61.5000", "purpose": "Порог заказа = lead_time*daily_demand + safety_stock"},
        {"name": "target_stock", "type": "numeric(18,4)", "range": "≥ reorder_point", "example": "120.0000", "purpose": "Целевой остаток после поставки"},
        {"name": "reorder_qty", "type": "numeric(18,4)", "range": "≥ 0, кратно multiple", "example": "84.0000", "purpose": "Сколько надо заказать сейчас"},
        {"name": "calculated_at", "type": "timestamptz", "range": "ISO-8601", "example": "2026-05-08T04:01:30Z", "purpose": "Время расчёта"},
    ],
}

_REPLENISHMENT_PLANS: EntitySpec = {
    "name": "forecast.replenishment_plans",
    "title": "replenishment_plans — агрегаты по поставщику",
    "description": "Готовый план заказов. Approved-планы становятся PO в M6.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "11111111-2222-3333-4444-555555555555", "purpose": "PK плана"},
        {"name": "run_id", "type": "UUID", "range": "FK forecast_runs", "example": "9f8e7d6c-...", "purpose": "К какому прогону"},
        {"name": "supplier_id", "type": "string", "range": "FK supplier", "example": "S-042", "purpose": "Кто поставит"},
        {"name": "location_id", "type": "string", "range": "FK location", "example": "L-001", "purpose": "Куда"},
        {"name": "plan_date", "type": "date", "range": "YYYY-MM-DD", "example": "2026-05-15", "purpose": "Желаемая дата поставки"},
        {"name": "total_qty", "type": "numeric(18,4)", "range": "≥ 0", "example": "240.0000", "purpose": "Суммарное количество всех позиций"},
        {"name": "lines_count", "type": "int", "range": "≥ 0", "example": "8", "purpose": "Сколько позиций в плане"},
        {"name": "status", "type": "string", "range": "'draft' | 'approved' | 'cancelled'", "example": "approved", "purpose": "Статус плана"},
        {"name": "approved_at", "type": "timestamptz | null", "range": "ISO-8601", "example": "2026-05-08T05:00:00Z", "purpose": "Когда одобрен"},
        {"name": "approved_by", "type": "string | null", "range": "user-id", "example": "ops_lead", "purpose": "Кто одобрил"},
        _F_CREATED_AT,
    ],
}

ALL_FORECAST_ENTITIES: list[EntitySpec] = [
    _FORECAST_RUNS,
    _FORECASTS,
    _CALCULATION_LINES,
    _REPLENISHMENT_PLANS,
]


# --- m6 (orders schema) ------------------------------------------------------

_PURCHASE_ORDERS: EntitySpec = {
    "name": "orders.purchase_orders",
    "title": "purchase_orders — заказы поставщикам",
    "description": "Партиционирована monthly по created_at. PK (id, created_at). Один план → один не-cancelled PO.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "abcdef12-3456-7890-abcd-ef1234567890", "purpose": "PK заказа"},
        {"name": "po_number", "type": "string", "range": "PO-YYYYMMDD-NNNNNN", "example": "PO-20260508-000042", "purpose": "Человеко-читаемый номер заказа"},
        {"name": "plan_id", "type": "UUID", "range": "FK replenishment_plans.id", "example": "11111111-2222-3333-4444-555555555555", "purpose": "На основании какого плана создан"},
        {"name": "supplier_id", "type": "string", "range": "FK supplier", "example": "S-042", "purpose": "Поставщик"},
        {"name": "location_id", "type": "string", "range": "FK location", "example": "L-001", "purpose": "Куда поставка"},
        {"name": "status", "type": "string", "range": "'draft' | 'ready_to_send' | 'sent' | 'confirmed_by_erp' | 'received' | 'cancelled'", "example": "ready_to_send", "purpose": "Статус workflow"},
        {"name": "total_qty", "type": "numeric(18,4)", "range": "≥ 0", "example": "240.0000", "purpose": "Суммарное количество всех po_lines"},
        {"name": "total_amount", "type": "numeric(18,4) | null", "range": "≥ 0", "example": "69480.0000", "purpose": "Сумма заказа в currency"},
        {"name": "currency", "type": "string", "range": "ISO 4217, default 'UAH'", "example": "UAH", "purpose": "Валюта заказа"},
        {"name": "delivery_date", "type": "date | null", "range": "YYYY-MM-DD", "example": "2026-05-15", "purpose": "Ожидаемая дата поставки = today + lead_time"},
        {"name": "notes", "type": "string | null", "range": "произвольный текст", "example": "Срочно — праздник", "purpose": "Комментарий"},
        {"name": "sent_at", "type": "timestamptz | null", "range": "ISO-8601", "example": "2026-05-08T06:30:00Z", "purpose": "Когда отправлен (M7)"},
        {"name": "sent_to_channel", "type": "string | null", "range": "channel_type", "example": "erp_api", "purpose": "Через какой канал отправлен"},
        {"name": "cancel_reason", "type": "string | null", "range": "произвольный текст", "example": "supplier_unavailable", "purpose": "Причина отмены (если cancelled)"},
        _F_CREATED_AT,
        _F_UPDATED_AT,
    ],
}

_PO_LINES: EntitySpec = {
    "name": "orders.po_lines",
    "title": "po_lines — позиции заказа",
    "description": "Каждая строка = один товар в PO. unit_price резолвится через pricing waterfall.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "fedcba98-7654-3210-fedc-ba9876543210", "purpose": "PK строки"},
        {"name": "po_id", "type": "UUID", "range": "FK purchase_orders.id", "example": "abcdef12-...", "purpose": "К какому PO"},
        {"name": "product_id", "type": "string", "range": "FK products", "example": "P-00042", "purpose": "Товар"},
        {"name": "qty", "type": "numeric(18,4)", "range": "> 0", "example": "84.0000", "purpose": "Количество"},
        {"name": "unit_price", "type": "numeric(18,4) | null", "range": "≥ 0", "example": "289.50", "purpose": "Цена за единицу"},
        {"name": "line_amount", "type": "numeric(18,4) | null", "range": "= qty * unit_price", "example": "24318.0000", "purpose": "Сумма строки"},
        {"name": "pricing_source", "type": "string | null", "range": "'product' | 'supplier_default' | 'missing'", "example": "product", "purpose": "Откуда взялась цена (waterfall)"},
        {"name": "notes", "type": "string | null", "range": "текст", "example": "новая упаковка", "purpose": "Комментарий"},
        _F_CREATED_AT,
    ],
}

_PO_STATUS_HISTORY: EntitySpec = {
    "name": "orders.po_status_history",
    "title": "po_status_history — audit статусов PO",
    "description": "Каждый переход статуса = строка. Используется для отладки workflow.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "00000000-1111-2222-3333-444444444444", "purpose": "PK события"},
        {"name": "po_id", "type": "UUID", "range": "FK purchase_orders.id", "example": "abcdef12-...", "purpose": "К какому PO"},
        {"name": "from_status", "type": "string | null", "range": "статус PO, NULL для первого", "example": "draft", "purpose": "Из какого статуса перешли"},
        {"name": "to_status", "type": "string", "range": "статус PO", "example": "ready_to_send", "purpose": "В какой статус"},
        {"name": "reason", "type": "string | null", "range": "произвольный текст", "example": "auto_promote", "purpose": "Причина перехода"},
    ],
}

ALL_ORDER_ENTITIES: list[EntitySpec] = [_PURCHASE_ORDERS, _PO_LINES, _PO_STATUS_HISTORY]


# --- m7 (channels schema) ----------------------------------------------------

_SUPPLIER_CHANNEL_CONFIG: EntitySpec = {
    "name": "channels.supplier_channel_config",
    "title": "supplier_channel_config — конфиг канала на поставщика",
    "description": "1 запись per supplier. Определяет endpoint, формат и аутентификацию.",
    "fields": [
        {"name": "supplier_id", "type": "string", "range": "PK, FK supplier", "example": "S-042", "purpose": "Поставщик"},
        {"name": "channel_type", "type": "string", "range": "'erp_api' | 'edi_x12' | 'edi_edifact' | '1c_xml' | 'crm'", "example": "erp_api", "purpose": "Протокол отправки"},
        {"name": "endpoint_url", "type": "string", "range": "URL", "example": "https://erp.supplier-x.com/api/v1/orders", "purpose": "Куда POST'ить заказ"},
        {"name": "auth_mode", "type": "string", "range": "'api_key' | 'oauth2' | 'mtls' | 'none'", "example": "api_key", "purpose": "Метод аутентификации"},
        {"name": "auth_credentials_ref", "type": "string | null", "range": "vault/secret reference", "example": "vault://kv/supplier-x/api-key", "purpose": "Где лежат секреты (НЕ значение)"},
        {"name": "timeout_sec", "type": "int", "range": "> 0, default 30", "example": "30", "purpose": "HTTP timeout"},
        {"name": "retry_max", "type": "int", "range": "≥ 0, default 3", "example": "3", "purpose": "Сколько попыток при ошибке"},
        {"name": "is_active", "type": "bool", "range": "true/false", "example": "true", "purpose": "Активен ли канал"},
        _F_CREATED_AT,
        _F_UPDATED_AT,
    ],
}

_SEND_ATTEMPTS: EntitySpec = {
    "name": "channels.send_attempts",
    "title": "send_attempts — журнал попыток отправки",
    "description": "Партиционирована monthly по started_at. Идемпотентность через external_ref.",
    "fields": [
        {"name": "id", "type": "UUID", "range": "RFC 4122", "example": "deadbeef-1234-5678-9abc-def012345678", "purpose": "PK попытки"},
        {"name": "po_id", "type": "UUID", "range": "FK purchase_orders.id", "example": "abcdef12-...", "purpose": "Какой PO отправляли"},
        {"name": "supplier_id", "type": "string", "range": "FK supplier", "example": "S-042", "purpose": "Поставщик"},
        {"name": "channel_type", "type": "string", "range": "channel_type из config", "example": "erp_api", "purpose": "Через какой канал"},
        {"name": "started_at", "type": "timestamptz", "range": "ISO-8601 (партиционный ключ)", "example": "2026-05-08T06:30:00Z", "purpose": "Когда начали отправлять"},
        {"name": "finished_at", "type": "timestamptz | null", "range": "ISO-8601", "example": "2026-05-08T06:30:01Z", "purpose": "Когда закончили (NULL если pending)"},
        {"name": "status", "type": "string", "range": "'pending' | 'success' | 'failed' | 'skipped'", "example": "success", "purpose": "Результат попытки"},
        {"name": "http_status_code", "type": "int | null", "range": "100–599", "example": "200", "purpose": "HTTP код ответа ERP"},
        {"name": "request_body", "type": "string | null", "range": "JSON, обрезается до 8KB", "example": '{"po_number":"PO-20260508-000042","lines":[...]}', "purpose": "Что отправили (для отладки)"},
        {"name": "response_body", "type": "string | null", "range": "string, до 8KB", "example": '{"id":"received-001","status":"accepted"}', "purpose": "Что вернул ERP"},
        {"name": "error_message", "type": "string | null", "range": "текст", "example": "connection timeout after 30s", "purpose": "Описание ошибки (если failed)"},
        {"name": "retry_count", "type": "int", "range": "≥ 0", "example": "0", "purpose": "Какая по счёту попытка"},
        {"name": "external_ref", "type": "string | null", "range": "ID на стороне ERP", "example": "received-001", "purpose": "Идемпотентность (один success per po_id)"},
    ],
}

ALL_CHANNEL_ENTITIES: list[EntitySpec] = [_SUPPLIER_CHANNEL_CONFIG, _SEND_ATTEMPTS]


# --- ERP request body (M7 → mock-erp) ----------------------------------------

_ERP_RECEIVED_ORDER: EntitySpec = {
    "name": "POST /api/v1/orders (request body)",
    "title": "Body POST'а в mock-erp — то, что M7 отправляет",
    "description": "JSON тело отправляется в endpoint клиента (mock-erp в тесте). Каждый supplier ожидает свой формат — здесь приведён формат erp_api channel_type.",
    "fields": [
        {"name": "po_number", "type": "string", "range": "PO-YYYYMMDD-NNNNNN", "example": "PO-20260508-000042", "purpose": "Идентификатор заказа на стороне клиента"},
        {"name": "supplier_id", "type": "string", "range": "ID поставщика", "example": "S-042", "purpose": "Кому адресован заказ"},
        {"name": "location_id", "type": "string", "range": "ID локации доставки", "example": "L-001", "purpose": "Куда доставить"},
        {"name": "delivery_date", "type": "date", "range": "YYYY-MM-DD", "example": "2026-05-15", "purpose": "Когда нужна поставка"},
        {"name": "currency", "type": "string", "range": "ISO 4217", "example": "UAH", "purpose": "Валюта"},
        {"name": "lines", "type": "array<object>", "range": "≥ 1 элемент", "example": '[{"product_id":"P-00042","qty":84,"unit_price":289.50}]', "purpose": "Позиции заказа: product_id, qty, unit_price"},
        {"name": "external_ref", "type": "string", "range": "any string", "example": "ezoo-attempt-deadbeef", "purpose": "Idempotency key (один и тот же ref = один заказ в ERP)"},
    ],
}


# --- Module → input/output entity registry -----------------------------------

# Для каждого модуля указываем:
#   * full: список EntitySpec с полным разбором полей (отображаются раскрытыми)
#   * compact: tuples (name, description) для остальных сущностей —
#              отображаются compact, без таблицы полей.

class ModuleSpec(TypedDict):
    input_full: list[EntitySpec]
    input_compact: list[tuple[str, str]]
    output_full: list[EntitySpec]
    output_compact: list[tuple[str, str]]


def _compact_others(full_names: set[str], all_entities: list[EntitySpec]) -> list[tuple[str, str]]:
    """Return (name, description) tuples for entities not in full_names."""
    return [(e["name"], e["description"]) for e in all_entities if e["name"] not in full_names]


_ERP_FULL_NAMES = {_PRODUCTS["name"], _RECEIPT_LINE["name"], _SUPPLY_SPEC["name"]}
_MART_FULL_NAMES = {_MART_CALCULATION_INPUT["name"], _MART_DEMAND_HISTORY["name"]}

MODULE_FIELD_SPECS: dict[str, ModuleSpec] = {
    "m0": {
        "input_full": [],
        "input_compact": [
            ("Faker (mock-erp self-generation)", "Mock-erp генерирует данные сам при старте через Faker (90 дней истории). Нет внешних входов."),
            ("POST /api/v1/orders (от M7)", "Принимает входящие заказы от Channel Router — см. описание поля POST body в /m7."),
        ],
        "output_full": [_PRODUCTS, _RECEIPT_LINE, _SUPPLY_SPEC],
        "output_compact": _compact_others(_ERP_FULL_NAMES, ALL_ERP_ENTITIES),
    },
    "m1": {
        "input_full": [_PRODUCTS, _RECEIPT_LINE, _SUPPLY_SPEC],
        "input_compact": _compact_others(_ERP_FULL_NAMES, ALL_ERP_ENTITIES),
        "output_full": [_PRODUCTS, _RECEIPT_LINE, _SUPPLY_SPEC],
        "output_compact": _compact_others(_ERP_FULL_NAMES, ALL_ERP_ENTITIES),
    },
    "m2": {
        "input_full": [_RECEIPT_LINE],
        "input_compact": [
            ("public.* (16 таблиц от M1)", "DTO как в /m1 output. ETL читает через NDJSON streaming endpoints source-adapter."),
        ],
        "output_full": [_MART_CALCULATION_INPUT, _MART_DEMAND_HISTORY],
        "output_compact": _compact_others(_MART_FULL_NAMES, ALL_MART_ENTITIES),
    },
    "m3": {
        "input_full": [_MART_CALCULATION_INPUT, _MART_DEMAND_HISTORY],
        "input_compact": _compact_others(_MART_FULL_NAMES, ALL_MART_ENTITIES),
        "output_full": [],
        "output_compact": [
            ("REST JSON (всё содержимое marts.* + cursor pagination)",
             "Поля совпадают с marts.* (см. таблицы выше). Дополнительно ответ содержит заголовок Link с next-cursor для пагинации."),
        ],
    },
    "m4": {
        "input_full": [_MART_CALCULATION_INPUT, _MART_DEMAND_HISTORY],
        "input_compact": [(_MART_SUPPLIER_SCORECARD["name"], _MART_SUPPLIER_SCORECARD["description"])],
        "output_full": [_KPI_SNAPSHOTS],
        "output_compact": [(_KPI_CALIBRATIONS["name"], _KPI_CALIBRATIONS["description"])],
    },
    "m5": {
        "input_full": [_MART_CALCULATION_INPUT, _MART_DEMAND_HISTORY, _KPI_SNAPSHOTS],
        "input_compact": [],
        "output_full": [_FORECASTS, _REPLENISHMENT_PLANS],
        "output_compact": [
            (_FORECAST_RUNS["name"], _FORECAST_RUNS["description"]),
            (_CALCULATION_LINES["name"], _CALCULATION_LINES["description"]),
        ],
    },
    "m6": {
        "input_full": [_REPLENISHMENT_PLANS],
        "input_compact": [
            (_FORECAST_RUNS["name"], _FORECAST_RUNS["description"]),
        ],
        "output_full": [_PURCHASE_ORDERS, _PO_LINES],
        "output_compact": [(_PO_STATUS_HISTORY["name"], _PO_STATUS_HISTORY["description"])],
    },
    "m7": {
        "input_full": [_PURCHASE_ORDERS, _SUPPLIER_CHANNEL_CONFIG],
        "input_compact": [],
        "output_full": [_SEND_ATTEMPTS, _ERP_RECEIVED_ORDER],
        "output_compact": [],
    },
}


def get_module_spec(module_key: str) -> ModuleSpec | None:
    """Return field specs for a module key like 'm0', 'm1', ...; None if not found."""
    return MODULE_FIELD_SPECS.get(module_key)
