"""Подробные русскоязычные описания модулей pipeline для дашбордов.

Каждый модуль описывает:
- title       — человекочитаемое название
- purpose     — назначение модуля (что и зачем делает)
- input_from  — откуда берёт данные
- input_desc  — что именно читает (формат, способ, авторизация)
- process     — что делает с данными (трансформации, агрегаты, валидация)
- output_to   — куда передаёт данные дальше
- output_desc — формат выходных данных, как их получает следующий модуль
- next_step   — короткое описание следующего шага в pipeline
- card_short  — короткое описание (2–3 строки) для карточки на index-странице

Описания используются на всех страницах /m{N} и на index. Цифры (counts, latest run)
подставляются на странице из реального состояния БД и не дублируются здесь.
"""
from __future__ import annotations

from typing import TypedDict


class ModuleDescription(TypedDict):
    """Структура описания одного модуля pipeline."""

    title: str
    purpose: str
    input_from: str
    input_desc: str
    process: str
    output_to: str
    output_desc: str
    next_step: str
    card_short: str


PIPELINE_OVERVIEW: str = (
    "E-Zoo Replenishment Pipeline — система автоматического пополнения товарных "
    "запасов. 7 микросервисов на Go + 1 mock-сервис ERP клиента на Python. "
    "Полный цикл: чтение данных из ERP → валидация и трансформация в data marts → "
    "расчёт KPI → прогноз спроса → построение заказов → отправка обратно в ERP."
)


MODULE_DESCRIPTIONS: dict[str, ModuleDescription] = {
    "m0": {
        "title": "Mock ERP (Source)",
        "purpose": (
            "Имитация ERP-системы клиента E-Zoo для тестирования pipeline. "
            "Содержит синтетические данные о товарах, продажах, остатках и "
            "поставщиках за период 90 дней. Корень всего pipeline — все "
            "downstream-модули в конечном счёте читают данные отсюда."
        ),
        "input_from": "—",
        "input_desc": (
            "Сущности генерируются Faker-ом при первом старте контейнера и "
            "хранятся в памяти процесса. Нет внешних источников; mock-erp "
            "также принимает обратно готовые заказы от Channel Router (M7) "
            "через POST /api/v1/orders."
        ),
        "process": (
            "Хранит 16 типов сущностей: master-данные (products, suppliers, "
            "locations, categories, supply_spec, order_rule и др.) и факты "
            "(receipt_line, stock_movement, location_stock_snapshot). "
            "Отдаёт каждую сущность через REST с авторизацией по X-API-Key и "
            "заголовком X-Total-Count в ответе."
        ),
        "output_to": "Source Adapter (M1)",
        "output_desc": (
            "16 типов сущностей через REST API GET /api/v1/{entity}. M1 ходит "
            "за ними один раз в сутки (cron 02:00). Принимает также входящие "
            "заказы от M7 в endpoint POST /api/v1/orders."
        ),
        "next_step": (
            "Source Adapter (M1) забирает данные из mock-erp через HTTP-pull "
            "раз в сутки и сохраняет в public.* схему PostgreSQL."
        ),
        "card_short": (
            "Mock ERP клиента — синтетические master-данные и факты за 90 дней. "
            "Корень pipeline."
        ),
    },
    "m1": {
        "title": "Source Adapter",
        "purpose": (
            "Ежедневная джоба (cron 02:00 Europe/Kyiv) забирает данные из ERP "
            "клиента (mock-erp) через HTTP, валидирует severity-движком "
            "(правила в validation_rules.yaml), сохраняет в public.* таблицы "
            "PostgreSQL и атомарно flip-ает snapshot_pointer для "
            "консистентного снимка."
        ),
        "input_from": "Mock ERP (M0)",
        "input_desc": (
            "16 сущностей через REST API mock-erp. Каждый вызов "
            "GET /api/v1/{entity} с заголовком X-API-Key; в ответе "
            "X-Total-Count и тело со всеми строками."
        ),
        "process": (
            "Мапит ERP DTO в внутренний формат, фильтрует by severity "
            "(critical → reject_log), UPSERT-ит мастер-данные, INSERT-ит "
            "факты в partitioned-таблицы по event_date. После успешной "
            "загрузки atomically обновляет snapshot_pointer (current_load_id) "
            "— downstream-потребители видят данные одного консистентного снимка."
        ),
        "output_to": "ETL Validation (M2)",
        "output_desc": (
            "Своё HTTP API GET /v1/{entity} в формате NDJSON streaming с "
            "JWT-авторизацией ролью x-flow-etl. Поддерживает chunked-чтение "
            "и pagination через cursor."
        ),
        "next_step": (
            "ETL (M2) забирает свежие данные через API source-adapter и строит "
            "5 mart-таблиц для аналитики и расчётов."
        ),
        "card_short": (
            "Cron 02:00 — pull из mock-erp, severity-валидация, UPSERT в public.*. "
            "Atomic snapshot flip."
        ),
    },
    "m2": {
        "title": "ETL Validation",
        "purpose": (
            "Ежедневная джоба (cron 02:30) забирает 16 сущностей из API "
            "source-adapter, прогоняет cross-entity валидацию (FK consistency, "
            "unique business keys, aggregate matches), строит 5 mart-таблиц — "
            "денормализованных и оптимизированных под аналитические запросы."
        ),
        "input_from": "Source Adapter (M1)",
        "input_desc": (
            "HTTP NDJSON streaming с JWT (роль x-flow-etl). Все 16 сущностей "
            "загружаются в рамках одного source_load_id для атомарного "
            "снимка — если M1 успел сделать flip между чтениями, ETL "
            "получит ошибку version-mismatch и переначнёт run."
        ),
        "process": (
            "Загружает в staging-таблицы (TEMP), валидирует cross-entity "
            "rules (5 встроенных типов), строит mart_master_current "
            "(текущие master-данные), mart_demand_history (агрегации "
            "продаж), mart_calculation_input (готовые данные для калькулятора "
            "с pre-resolved supply_spec/order_rule), mart_kpi_daily, "
            "mart_supplier_scorecard. Atomic flip всех mart в одной транзакции."
        ),
        "output_to": "Data Marts (M3), KPI (M4), Forecast (M5)",
        "output_desc": (
            "5 mart-таблиц в схеме marts.*. Data Marts API (M3) отдаёт их по "
            "HTTP сторонним потребителям; KPI (M4) и Forecast (M5) читают "
            "напрямую через DB с ролью mart_reader."
        ),
        "next_step": (
            "KPI (M4) и Forecast (M5) читают marts.* напрямую и считают "
            "OSA/OTIF/Stock Days и 60-дневный прогноз спроса."
        ),
        "card_short": (
            "Cron 02:30 — cross-entity валидация и сборка 5 mart-таблиц "
            "из public.* в marts.*."
        ),
    },
    "m3": {
        "title": "Data Marts API",
        "purpose": (
            "Read-only REST API над marts-таблицами для внешних потребителей "
            "(KPI, Forecast и сторонних клиентов в будущем). Storage abstraction "
            "interface — позволит в дальнейшем заменить PostgreSQL на "
            "ClickHouse/Parquet без поломки потребителей."
        ),
        "input_from": "ETL Validation (M2) — через DB",
        "input_desc": (
            "Прямой read из marts.* таблиц с ролью mart_reader (read-only). "
            "Запросы инкапсулированы в storage interface; возможна миграция "
            "на ClickHouse без изменения API."
        ),
        "process": (
            "Отдаёт NDJSON streaming с cursor-pagination и ETag headers. "
            "Кэширует current snapshot версии 60s — повторные запросы за "
            "одной и той же версией не бьют по БД."
        ),
        "output_to": "Любые внешние потребители (BI, Forecast, KPI)",
        "output_desc": (
            "GET /v1/marts/{name} — NDJSON streaming с cursor-pagination "
            "(JWT roles: x-flow-etl, it-read). GET /v1/marts/{name}/version "
            "возвращает текущий etl_run_id и committed_at."
        ),
        "next_step": (
            "Через API могут читать любые BI-системы или сторонние сервисы; "
            "internal-потребители (KPI, Forecast) предпочитают прямой DB read."
        ),
        "card_short": (
            "Read-only HTTP-фасад над marts.* с ETag и cursor-pagination. "
            "Storage abstraction для будущей миграции."
        ),
    },
    "m4": {
        "title": "KPI & Calibration",
        "purpose": (
            "Ежедневная джоба (cron 04:00) считает три KPI — OSA "
            "(On-Shelf Availability), OTIF (On-Time In-Full), Stock Days — "
            "для каждой пары (product, location). Применяет hierarchical "
            "калибровки (product_location → location → supplier → category → "
            "global override)."
        ),
        "input_from": "ETL marts (M2) — через DB",
        "input_desc": (
            "Прямой DB read из marts.mart_demand_history, "
            "marts.mart_calculation_input и marts.mart_supplier_scorecard. "
            "Ходит за консистентным snapshot одного etl_run_id."
        ),
        "process": (
            "Для каждого product×location: OSA = sum(time_in_stock) / "
            "total_observation_time; OTIF = on-time-and-full orders / "
            "total orders; Stock Days = current_stock / avg_daily_demand_30d. "
            "Записывает в kpi.kpi_snapshots (partitioned by as_of_date "
            "месячно). Калибровки применяются по приоритету: специфичная "
            "перебивает общую."
        ),
        "output_to": "Forecast (M5), сторонние BI-консумеры",
        "output_desc": (
            "kpi.kpi_snapshots (partitioned monthly) + kpi.kpi_calibrations. "
            "Доступно через REST /v1/kpi/snapshots для внешних потребителей."
        ),
        "next_step": (
            "Forecast (M5) использует KPI (особенно Stock Days) при "
            "выборе critical product×location пар для приоритезации."
        ),
        "card_short": (
            "Cron 04:00 — OSA, OTIF, Stock Days для product×location. "
            "Hierarchical калибровки."
        ),
    },
    "m5": {
        "title": "Forecast Engine",
        "purpose": (
            "Ежедневная джоба (cron 05:00) строит 60-дневный прогноз спроса "
            "для каждой пары (product, location) через SMA30 × DOW × WOY "
            "multipliers. Calculator вычисляет reorder_point "
            "(safety_stock + lead_time × demand) и reorder_qty. Constructor "
            "группирует по supplier и применяет MOQ + multiplier rounding для "
            "формирования replenishment_plans (status=draft)."
        ),
        "input_from": "ETL marts (M2) + KPI (M4) — через DB",
        "input_desc": (
            "Прямой DB read из marts.mart_demand_history (история продаж), "
            "marts.mart_calculation_input (current stock + supplier rules) и "
            "kpi.kpi_snapshots. Без HTTP — производительность важна на "
            "больших фан-аутах."
        ),
        "process": (
            "Forecaster.Predict для каждой пары → ~60 forecast строк "
            "(SMA30 × DOW × WOY). Calculator считает reorder_point и "
            "reorder_qty. Constructor группирует по supplier_id, применяет "
            "MOQ и multiplier rounding, формирует replenishment_plans с "
            "lines (один план = один поставщик + набор позиций) в "
            "status=draft."
        ),
        "output_to": "Order Builder (M6)",
        "output_desc": (
            "forecast.forecasts (прогнозы на 60 дней) и "
            "forecast.replenishment_plans (планы пополнения, status=draft). "
            "Также REST /v1/forecast/runs/* и /v1/replenishment/plans/* для "
            "внешних потребителей."
        ),
        "next_step": (
            "Order Builder (M6) подбирает approved replenishment_plans "
            "и конвертирует их в полноценные purchase orders."
        ),
        "card_short": (
            "Cron 05:00 — 60-дневный прогноз SMA30×DOW×WOY, reorder-point, "
            "MOQ, replenishment_plans."
        ),
    },
    "m6": {
        "title": "Order Builder",
        "purpose": (
            "Ежедневная джоба (cron 06:00) подбирает approved "
            "replenishment_plans, конвертирует их в полноценные purchase "
            "orders с уникальными PO numbers (формат PO-YYYYMMDD-NNNNNN), "
            "рассчитывает delivery_date от supplier.lead_time_days, применяет "
            "pricing waterfall (products.unit_price → "
            "supplier.default_unit_price → NULL)."
        ),
        "input_from": "Forecast (M5) — через DB",
        "input_desc": (
            "Прямой DB read из forecast.replenishment_plans WHERE "
            "status='approved' с join на forecast.replenishment_plan_lines "
            "и products/suppliers."
        ),
        "process": (
            "Для каждого approved plan создаёт purchase_order и po_lines в "
            "одной транзакции. PO number генерится атомарно через sequence. "
            "Status workflow: draft → ready_to_send → sent → "
            "confirmed_by_erp → received | cancelled. На каждый переход — "
            "запись в orders.po_status_history."
        ),
        "output_to": "Channel Router (M7)",
        "output_desc": (
            "orders.purchase_orders + orders.po_lines. M7 подбирает POs "
            "со status='ready_to_send' через DB read."
        ),
        "next_step": (
            "Channel Router (M7) забирает POs со status=ready_to_send и "
            "отправляет их в ERP клиента через канальный endpoint."
        ),
        "card_short": (
            "Cron 06:00 — approved plans → purchase_orders с уникальными "
            "PO numbers, delivery_date, pricing waterfall."
        ),
    },
    "m7": {
        "title": "Channel Router",
        "purpose": (
            "Ежедневная джоба (cron 06:30) подбирает purchase_orders со "
            "status='ready_to_send', резолвит channel-конфиг для каждого "
            "supplier, формирует body через BodyFormatter (JSON для erp_api), "
            "отправляет через ChannelSender (MVP: ErpAPISender) с "
            "retry/backoff cap 30s. Логирует attempt и обновляет статус PO."
        ),
        "input_from": "Order Builder (M6) — через DB",
        "input_desc": (
            "Прямой DB read из orders.purchase_orders WHERE "
            "status='ready_to_send' и channels.supplier_channel_config "
            "(endpoint URL, auth credentials)."
        ),
        "process": (
            "Per-PO транзакция: SELECT FOR UPDATE → InsertAttempt → "
            "Send (HTTP POST с api-key/JWT/oauth2) → FinishAttempt → "
            "MarkPOSent. Idempotency через external_ref если retry на уже "
            "принятом PO. На failure — exponential backoff с cap 30s; "
            "после исчерпания попыток PO остаётся в ready_to_send для "
            "следующего запуска."
        ),
        "output_to": "ERP клиента (mock-erp в тесте)",
        "output_desc": (
            "POST /api/v1/orders в mock-erp (или реальный ERP в prod). "
            "Каждая попытка отправки логируется в channels.send_attempts "
            "со статусом success/error. PO-status переходит в sent при "
            "успехе."
        ),
        "next_step": (
            "Mock ERP принимает заказы и возвращает их через "
            "GET /api/v1/orders/received — конец pipeline (loop замкнулся)."
        ),
        "card_short": (
            "Cron 06:30 — ready_to_send POs → POST в ERP клиента, "
            "send_attempts, retry с cap 30s."
        ),
    },
}


def get_description(slug: str) -> ModuleDescription | None:
    """Получить описание модуля по slug (m0..m7)."""
    return MODULE_DESCRIPTIONS.get(slug)
