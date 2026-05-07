# Spec: data-marts (Модуль 3)
**Дата:** 2026-05-07
**Tier:** M
**Mode:** compact

## Решения (defaulted на основе контекста, без интервью с пользователем — auto mode)

| Q | Решение |
|---|---|
| Q-001 Binary | Встраиваем в cmd/source-adapter как новую feature data_marts. НЕ отдельный binary. |
| Q-002 Cache TTL | 60s, ключ = (mart_name, current_etl_run_id). Invalidate-by-version (cheap). |
| Q-003 Pagination | Cursor-based (consistent с Модулем 1). Cursor = (etl_run_id, last_seen_pk). |
| Q-004 Authorization | JWT с ролями `x-flow-etl` + `it-read` (read-only). Без admin endpoints. |
| Q-005 Schema endpoint | Список колонок с типами + PK + provenance metadata (etl_run_id, source_load_id, committed_at). |
| Q-006 Response format | NDJSON streaming (consistent с Модулем 1). Inline для маленьких mart (mart_master_current ~< 1MB). |
| Q-007 Storage abstraction | Интерфейс `MartReader` внутри feature data_marts, без выноса в pkg/. YAGNI. Future ClickHouse/Parquet — просто новая реализация интерфейса. |

## Endpoints (4)
| Method | Path | Auth | Описание |
|---|---|---|---|
| GET | /v1/marts | x-flow-etl, it-read | Список доступных mart-таблиц + version |
| GET | /v1/marts/:name | x-flow-etl, it-read | NDJSON streaming строк mart-таблицы (current snapshot, cursor pagination) |
| GET | /v1/marts/:name/version | x-flow-etl, it-read | Текущий etl_run_id + committed_at |
| GET | /v1/marts/:name/schema | x-flow-etl, it-read | Схема mart-таблицы (колонки, типы, PK) |

## Цели MVP
1. **Stable read API** — потребители (Модули 4-5) ходят через единый интерфейс, не зная о структуре marts.*.
2. **Versioning** — каждый ответ содержит `X-Etl-Run-Id` header, позволяющий клиенту проверить актуальность.
3. **Future-proof** — интерфейс `MartReader` позволяет в будущем переключить storage без поломки потребителей.

## Открытые вопросы
Нет. Все решения defaulted.

## Non-goals MVP
- ClickHouse/Parquet implementations
- Querying with filters / SQL-like expressions
- Aggregation endpoints
- Subscription/push API
- Admin endpoints (CRUD над данными — это ответственность ETL)
