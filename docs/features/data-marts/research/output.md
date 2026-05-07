# Research: data-marts (Модуль 3)
**Дата:** 2026-05-07
**Mode:** compact (inline research, без subagent)

## Контекст
Модуль 3 «Витрины данных» из draft-plan: «Joba, которая будет ходить во второй модуль и получать оттуда данные и строить витрину данных. Делаем её максимально примитивной, плоской из нескольких таблиц, чтобы в дальнейшем при необходимости мы могли заменить с постгрыса, например, на кликхаус или на какой-то другой тип, или в паркете хранить.»

## Архитектурный конфликт с Модулем 2
Модуль 2 (etl-validation) уже создал 5 `marts.mart_*` таблиц: `mart_demand_history`, `mart_calculation_input`, `mart_kpi_daily`, `mart_master_current`, `mart_supplier_scorecard`. Они физически живут в той же БД source_adapter в schema `marts`.

Изначальный смысл Модуля 3 в draft-plan = «отдельная джоба, отдельный слой, swappable storage». В реальности Модуль 2 уже выполнил эту работу через ETL pipeline.

## Решение
Модуль 3 переориентируется на:
1. **Read-side API** — REST endpoints `GET /v1/marts/{name}` для потребителей (Модули 4 KPI, 5 forecast)
2. **Storage abstraction** — интерфейс `MartReader` с PG-имплементацией; контракт для будущей замены на ClickHouse/Parquet/DuckDB
3. **Data marts versioning** — endpoint `GET /v1/marts/{name}/version` возвращающий current `etl_run_id` + `committed_at`
4. **Cache layer** — In-memory cache для часто запрашиваемых mart-снимков

## Что уже есть в коде (после Модулей 1+2)
- 5 mart-таблиц в schema marts (готовы, наполняются ETL'ом)
- pgxpool, JWT, role middleware, slog, errorspkg, mappers паттерн
- Read-only role `mart_reader` (создана миграцией Модуля 2)
- etl_runs registry с источником данных

## Новый код
- internal/features/data_marts/ (handler/service/repository/router/sqls/models/dto/mappers/)
- Storage abstraction: pkg/martstore/ (MartReader interface) — для будущих CH/Parquet implementations
- Endpoints: GET /v1/marts/list, GET /v1/marts/{name}, GET /v1/marts/{name}/version, GET /v1/marts/{name}/schema
- Может встраиваться в существующий cmd/source-adapter (не отдельный binary — slim feature, вписывается в публичную API surface)

## Открытые вопросы
- Q-001: Куда встраивается feature data_marts — в cmd/source-adapter (через router) или в cmd/etl, или новый cmd/marts-api? Рекомендую cmd/source-adapter (уже отдаёт /v1/{entity}, расширим до /v1/marts/{name}).
- Q-002: Cache TTL — 0 (всегда свежий), 60s, configurable? Рекомендую 60s + invalidate при etl_run flip.
- Q-003: Pagination для GET /v1/marts/{name} — cursor-based как в Модуле 1, или offset/limit? Cursor consistent.
- Q-004: Authorization: x-flow-etl уже есть; добавить роль `mart-reader`? Рекомендую переиспользовать `x-flow-etl` + `it-read`.
- Q-005: Schema endpoint — JSON Schema, OpenAPI snippet, или просто список колонок с типами? Список колонок проще.
- Q-006: Какой формат ответа — NDJSON streaming (как в Модуле 1) или JSON-array? NDJSON для consistency.
- Q-007: Storage abstraction в pkg/martstore — действительно нужен сейчас, или достаточно интерфейса в feature и отрефакторим при появлении 2-го backend? Рекомендую интерфейс в feature (YAGNI).

## Tier: M (Medium)
- Endpoints: 4 (list, get, version, schema)
- Новых сущностей: 0 (читаем существующие mart_*)
- Миграций: 0
- Внешние интеграции: нет
- Затронутые фичи: 1
- Breaking changes API: нет
- Ожидаемый diff: 400-1000 LOC

→ M-tier: design 5-7 секций, без swimlane, plan = inline в design.
