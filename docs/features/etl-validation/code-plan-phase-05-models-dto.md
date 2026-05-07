# Phase 05 — Models / DTO

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Создать `package models` (domain) и `package dto` (Request/Response) для feature `etl_validation`. Без бизнес-логики — чистые struct-определения с тегами `db`, `json`, `validate`, `enums`, swag-аннотациями.

## Commit

```
feat(etl_validation): models and DTO structs (etl_run, reject_log, mart_*, admin)
```

## Files to CREATE

### Domain (`internal/features/etl_validation/models/`)

- `etl_run.go` — `EtlRun` struct (поля = колонки `marts.etl_runs` + go-friendly типы: `time.Time`, `*string`, `pgtype.UUID`, `json.RawMessage` для `marts_summary`).
- `reject.go` — `RejectLogEntry` struct (поля = колонки `marts.reject_log`).
- `audit.go` — `AuditAccessEntry` struct.
- `mart_demand_history.go` — domain-структура строки `mart_demand_history`.
- `mart_calculation_input.go` — domain-структура с `applicable_rule_id` и `applicable_rule_kind`.
- `mart_kpi_daily.go` — domain-структура.
- `mart_master_current.go` — domain-структура.
- `mart_supplier_scorecard.go` — domain-структура.
- `staging.go` — типы для записей staging-таблиц (входные сущности от source-adapter перед валидацией). Поля совпадают с DTO Модуля 1.

### DTO (`internal/features/etl_validation/models/dto/`)

- `post_etl_run_request.go`:
  ```go
  type PostEtlRunRequest struct {
      // пусто — POST /admin/etl-runs не имеет body (force start)
  }
  ```
- `retry_etl_run_request.go`:
  ```go
  type RetryEtlRunRequest struct {
      // пусто — body не требуется
  }
  ```
- `mart_refresh_request.go`:
  ```go
  type MartRefreshRequest struct {
      // body может быть пустым; имя mart берётся из path-param :name
  }
  ```
- `etl_run_response.go`:
  ```go
  type EtlRunResponse struct {
      ID            string          `json:"id"`
      StartedAt     time.Time       `json:"started_at"`
      FinishedAt    *time.Time      `json:"finished_at,omitempty"`
      CommittedAt   *time.Time      `json:"committed_at,omitempty"`
      Status        string          `json:"status" enums:"running,committed,failed,aborted"`
      Kind          string          `json:"kind"   enums:"full,mart_refresh"`
      TargetMart    *string         `json:"target_mart,omitempty"`
      SourceLoadID  *string         `json:"source_load_id,omitempty"`
      ParentRunID   *string         `json:"parent_run_id,omitempty"`
      Trigger       string          `json:"trigger" enums:"cron,admin,retry"`
      Requester     *string         `json:"requester,omitempty"`
      MartsSummary  json.RawMessage `json:"marts_summary,omitempty"`
      FailureReason *string         `json:"failure_reason,omitempty"`
      LinesTotal    int64           `json:"lines_total"`
      LinesFailed   int64           `json:"lines_failed"`
  }
  ```
- `etl_run_list_response.go` — пагинированный список + cursor.
- `reject_log_response.go` — `RejectLogEntryResponse` (включая `entity`, `severity` enum=`critical,soft`, `rule_id`, `row_payload`, `reason`).
- `reject_log_list_response.go`.
- `mart_refresh_response.go` — `{run_id, status, target_mart}`.

### Constants (`internal/features/etl_validation/constants/`)

- `constants.go` — package constants:
  ```go
  const (
      StatusRunning   = "running"
      StatusCommitted = "committed"
      StatusFailed    = "failed"
      StatusAborted   = "aborted"

      KindFull        = "full"
      KindMartRefresh = "mart_refresh"

      TriggerCron  = "cron"
      TriggerAdmin = "admin"
      TriggerRetry = "retry"

      SeverityCritical = "critical"
      SeveritySoft     = "soft"

      MartSupplierScorecard = "mart_supplier_scorecard"
      // ... остальные имена mart-ов
  )
  var (
      EtlRunStatuses = []string{StatusRunning, StatusCommitted, StatusFailed, StatusAborted}
      EtlRunKinds    = []string{KindFull, KindMartRefresh}
      EtlRunTriggers = []string{TriggerCron, TriggerAdmin, TriggerRetry}
      RejectSeverities = []string{SeverityCritical, SeveritySoft}
  )
  ```

## Files to MODIFY

- (нет)

## SQL / Migrations

Нет.

## Run after

```bash
go build ./internal/features/etl_validation/models/...
go vet ./internal/features/etl_validation/models/...
golangci-lint run ./internal/features/etl_validation/models/...
```

## Tests

- `models/dto/etl_run_response_swag_test.go` — sync-тест: значения `enums:"..."` ⇔ `constants.EtlRunStatuses`/`EtlRunKinds`/`EtlRunTriggers` (см. CLAUDE.md §8.1).
- `models/dto/reject_log_response_swag_test.go` — аналогично для severity.

## Definition of Done

- [ ] Все domain-структуры (`models/`) и DTO (`models/dto/`) созданы.
- [ ] `package models`, `package dto`, `package constants` корректно объявлены.
- [ ] Sync-тесты для enum-полей (`*_swag_test.go`) проходят.
- [ ] Swag-аннотации (`enums:`, `// @Description`) присутствуют на всех enum-полях.
- [ ] `go build ./...` проходит.
- [ ] `golangci-lint` без нарушений.

## Зависимости

Не требует БД, использует фазу 02 (sentinel errors уже определены).
