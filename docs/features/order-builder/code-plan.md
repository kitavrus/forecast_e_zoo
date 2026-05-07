# Code Plan: order-builder (Модуль 6)

**Tier:** M | **Phases:** 8 | **Atomic commit per phase**

| # | Фаза | Status | Файлы |
|---|------|--------|-------|
| 1 | Migration 4001 (schema orders + 3 таблицы + sequence + partitions) | completed | `internal/features/orders/sqls/migrations/4001_orders_schema.{up,down}.sql`, `embed.go` |
| 2 | Sentinel errors + DTO + models + constants | completed | `pkg/errorspkg/errors_orders.go`, `support_codes.go`, `internal/features/orders/{constants,models,models/dto}/...` |
| 3 | SQL queries + Repository | completed | `internal/features/orders/sqls/queries/*.sql`, `repository/*.go` |
| 4 | PONumberGenerator (sequence-based) | completed | `internal/features/orders/numbering/generator.go`+`_test.go` |
| 5 | POBuilder (pure function, plan→PO) | completed | `internal/features/orders/builder/po_builder.go`+`_test.go` |
| 6 | POBuilder service (orchestration + tx) | completed | `internal/features/orders/service/service.go`+`build.go`+`_test.go` |
| 7 | Scheduler (gocron 06:00 + advisory lock) | completed | `internal/features/orders/scheduler/scheduler.go`+`_test.go` |
| 8 | Handlers + mappers + router + DI + metrics + validation | completed | `internal/features/orders/handler/*.go`, `mappers/`, `router/`, `validators/`, observability metrics, `internal/app/app.go`, `internal/routers/routers.go`, `internal/config/config.go` |

## Quality gates per phase
- `go build ./...` — каждая фаза.
- `go test -race ./internal/features/orders/...` — фазы 4–8.
- `golangci-lint run ./internal/features/orders/...` — финал.

## Definition of Done (final)
- [x] Все 8 фаз завершены.
- [x] Все unit-тесты проходят (-race).
- [x] go build ./... зелёный.
- [x] golangci-lint run ./... без новых блокеров.
- [x] DI зарегистрирован в internal/app/app.go.
- [x] Router зарегистрирован в internal/routers/routers.go.
- [x] Prometheus метрики добавлены в observability.
