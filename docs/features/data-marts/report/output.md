# Pipeline Report: data-marts (Модуль 3)
**Дата:** 2026-05-07
**Профиль:** Бизнес-фича (Tier M, compact mode)
**Scope:** Read-only API + storage abstraction поверх marts.* (готовых из Модуля 2)

## Что сделано

Создан тонкий read-side для 5 mart-таблиц, наполняемых ETL'ом (Модуль 2). Потребители (Модули 4 KPI, 5 forecast) теперь ходят через стабильный REST + NDJSON streaming, не зная физической структуры marts.*. Интерфейс `MartReader` подготавливает почву для будущей замены backend'а (PG → ClickHouse/Parquet) без поломки потребителей.

## Артефакты
- docs/features/data-marts/research/output.md — research inline (без subagent)
- docs/features/data-marts/spec-interview/output.md — spec inline, 7 Q-NNN defaulted
- docs/features/data-marts/design.md — компактный design (один файл, 7 секций + 5 ADR)
- docs/features/data-marts/code-plan.md + code-plan-status.md + 6 фаз (inline)
- internal/features/data_marts/* — реализация
- 6 git коммитов (по одному на фазу)

## Endpoints

| Method | Path | Auth | Описание |
|---|---|---|---|
| GET | /v1/marts | x-flow-etl/it-read | Список 5 mart-таблиц + populated flag |
| GET | /v1/marts/:name | x-flow-etl/it-read | NDJSON streaming с cursor pagination, headers X-Etl-Run-Id / X-Next-Cursor |
| GET | /v1/marts/:name/version | x-flow-etl/it-read | Текущий committed etl_run_id + committed_at (cached 60s) |
| GET | /v1/marts/:name/schema | x-flow-etl/it-read | Схема mart-таблицы (hardcoded list per ADR-002) |

## Ключевые архитектурные решения (5 ADR)

- ADR-001: Cursor-based pagination (consistent с Модулем 1)
- ADR-002: Hardcoded schema для каждой mart (5 mart-таблиц с фиксированной структурой; динамический schema discovery — overengineering для MVP)
- ADR-003: Не вводим новые sentinel errors — переиспользуем ErrNotFound (mart_name unknown), ErrInvalidCursor (из Модуля 1)
- ADR-004: In-memory cache 60s TTL для GetVersion (часто запрашиваемая операция, версия меняется раз в сутки)
- ADR-005: MartReader interface внутри feature (не в pkg/) — YAGNI, выносим в pkg/ только при появлении 2-го backend

## Метрики прогона

- 1 subagent-вызов (compact)
- 6 git коммитов
- 0 миграций (читаем существующие marts.*)
- ~30 unit + 1 integration test

## Quality gates

- `go build ./...` — OK
- `go test -race -count=1 ./internal/features/data_marts/...` — OK
- `golangci-lint run` — 0 issues

## Известные ограничения (MVP)

- Только PG implementation MartReader. ClickHouse/Parquet — следующая итерация.
- Без filter/SQL-like queries (потребители получают full mart NDJSON).
- Без push/subscription API.
- Schema endpoint — hardcoded; динамическая интроспекция через pg_catalog — следующая итерация.

## Что дальше

Модули 4 (KPI/calibration), 5 (forecast-engine), 6 (order-builder), 7 (channel-routing) могут читать marts через GET /v1/marts/:name.
