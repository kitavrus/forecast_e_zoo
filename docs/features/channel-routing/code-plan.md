# Code Plan: channel-routing (Module 7) — 8 фаз

| # | Phase | Status | Files | Commit |
|---|---|---|---|---|
| 1 | Migration 5001 + embed | completed | `internal/features/channels/sqls/migrations/5001_channels_schema.{up,down}.sql`, `embed.go` | `feat(channels): phase 1 migration schema` |
| 2 | Sentinel errors + DTO + models | completed | `pkg/errorspkg/errors_channels.go`, `pkg/errorspkg/support_codes.go`, `internal/features/channels/{models/models.go,models/dto/dto.go,constants/constants.go}` | `feat(channels): phase 2 errors and models` |
| 3 | SQL queries + Repository + integration test | completed | `sqls/queries/*.sql`, `repository/repository.go`, `repository/queries.go`, `repository/integration_test.go` | `feat(channels): phase 3 repository and SQL` |
| 4 | ChannelSender interface + ErpAPIChannelSender + unit tests | completed | `sender/{sender.go,erp_api_sender.go,erp_api_sender_test.go}`, `auth/{provider.go,api_key.go,api_key_test.go}`, `formatter/{formatter.go,json.go,json_test.go}` | `feat(channels): phase 4 sender interface and ERP API impl` |
| 5 | ChannelRouter service (orchestration) | completed | `router_svc/{channel_router.go,channel_router_test.go}` | `feat(channels): phase 5 channel router service` |
| 6 | Scheduler (gocron 06:30 + advisory lock) | completed | `scheduler/{scheduler.go,scheduler_test.go}` | `feat(channels): phase 6 scheduler` |
| 7 | Service + Handlers + mappers + router + DI + metrics | completed | `service/{service.go,service_test.go}`, `handler/*.go`, `mappers/helpers.go`, `router/router.go`, `validators/{validators.go,validators_test.go}`, `internal/app/app.go`, `internal/routers/routers.go`, `internal/config/config.go`, `internal/observability/metrics.go` | `feat(channels): phase 7 service handlers router DI metrics` |
| 8 | Validation: build+lint+test | completed | (verification only) | `feat(channels): phase 8 validation` |

## Hard invariants
- Каждая фаза: `go build ./...` зелёный.
- Phase 1 → migration applied добавляет 2 таблицы.
- Phase 4 → unit-тесты `go test -race ./internal/features/channels/sender/...`.
- Phase 8 → quality gates все.

## Dependencies
- 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (строго последовательно).
