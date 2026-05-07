# Phase 04 — Migrations 1002 — etl_runs + reject_log + audit_access

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Создать миграцию `1002_etl_runs` — три служебные таблицы в `marts`: `etl_runs` (registry, аналог `loads` из Модуля 1), `reject_log` (отвергнутые валидацией строки), `audit_access` (логирование admin-обращений). Helper-партишн-функция (если применимо).

## Commit

```
feat(etl/migrations): 1002 etl_runs + reject_log + audit_access tables
```

## Files to CREATE

- `internal/features/etl_validation/sqls/migrations/1002_etl_runs.up.sql`:
  - `CREATE TABLE marts.etl_runs` — id (UUID PK), started_at, finished_at, committed_at, status (`running|committed|failed|aborted`), kind (`full|mart_refresh`), target_mart, source_load_id, parent_run_id (для retry), trigger (`cron|admin|retry`), requester, marts_summary (JSONB), failure_reason, lines_total, lines_failed, created_at, updated_at. Индексы: `(status)`, `(started_at DESC)`, `(source_load_id)`.
  - `CREATE TABLE marts.reject_log` — id (UUID PK), etl_run_id (FK → etl_runs), entity, severity (`critical|soft`), rule_id, row_payload (JSONB), reason, created_at. Индексы: `(etl_run_id)`, `(entity, severity)`, `(created_at DESC)`.
  - `CREATE TABLE marts.audit_access` — id (UUID PK), method, path, sub (JWT subject), role, request_id, status_code, latency_ms, accessed_at. Индекс: `(accessed_at DESC)`, `(sub)`.
  - GRANT SELECT на `marts.etl_runs`, `marts.reject_log`, `marts.audit_access` для роли `mart_reader` (только чтение).
- `internal/features/etl_validation/sqls/migrations/1002_etl_runs.down.sql`:
  - `DROP TABLE IF EXISTS marts.audit_access;`
  - `DROP TABLE IF EXISTS marts.reject_log;`
  - `DROP TABLE IF EXISTS marts.etl_runs;`

- `internal/features/etl_validation/sqls/migrations/1003_partition_maintenance.up.sql` (optional helper, добавляется только если решено реализовать в SQL, иначе оставить на Go-сторону):
  - `CREATE FUNCTION marts.fn_create_next_partition(table_regclass, target_month date) RETURNS void` — DDL для создания партиции на месяц.

> Если предпочитаем создавать партиции из Go в фазе 14 (scheduler pre-step) — `1003` можно опустить. Решение: реализуем в Go (см. design-integrations.md §3 partition maintenance), миграцию 1003 НЕ создаём.

## Files to MODIFY

- (нет)

## SQL / Migrations

См. `design-sql.md` §2. Полный DDL `etl_runs`:

```sql
CREATE TABLE marts.etl_runs (
    id              UUID PRIMARY KEY,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    committed_at    TIMESTAMPTZ,
    status          TEXT NOT NULL CHECK (status IN ('running','committed','failed','aborted')),
    kind            TEXT NOT NULL DEFAULT 'full' CHECK (kind IN ('full','mart_refresh')),
    target_mart     TEXT,                                  -- NULL для full, имя mart для mart_refresh
    source_load_id  UUID,
    parent_run_id   UUID REFERENCES marts.etl_runs(id),
    trigger         TEXT NOT NULL CHECK (trigger IN ('cron','admin','retry')),
    requester       TEXT,
    marts_summary   JSONB NOT NULL DEFAULT '{}'::jsonb,
    failure_reason  TEXT,
    lines_total     BIGINT NOT NULL DEFAULT 0,
    lines_failed    BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_etl_runs_status ON marts.etl_runs(status);
CREATE INDEX idx_etl_runs_started_at_desc ON marts.etl_runs(started_at DESC);
CREATE INDEX idx_etl_runs_source_load_id ON marts.etl_runs(source_load_id);
```

`reject_log` и `audit_access` — см. design-sql.md §2.

## Run after

```bash
make migrate-up-etl
psql $ETL_DSN -c "\dt marts.*"
make migrate-down-etl   # rollback 1002 only
make migrate-up-etl
```

## Tests

- Integration suite (фаза 06) применяет обе миграции (1001 + 1002).
- Smoke: `INSERT INTO marts.etl_runs(...)` от руки — успех.

## Definition of Done

- [ ] Миграции 1002 up/down созданы.
- [ ] CHECK constraints на `status`, `kind`, `trigger` корректны.
- [ ] FK `parent_run_id → etl_runs(id)` self-reference работает.
- [ ] Индексы созданы (`status`, `started_at DESC`, `source_load_id`).
- [ ] `make migrate-up-etl` (после 1001) проходит.
- [ ] `make migrate-down-etl` чисто откатывает.
- [ ] Роль `mart_reader` имеет SELECT на новые таблицы.

## Зависимости

Требует фазы 03 (schema marts уже создана).
