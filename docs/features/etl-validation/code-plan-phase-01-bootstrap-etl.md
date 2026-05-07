# Phase 01 — Bootstrap etl binary

> Статус — в [code-plan-status.md](./code-plan-status.md).

## Цель

Поднять каркас второго binary `cmd/etl` с graceful shutdown, DI root в `internal/etlapp/app.go`, env-конфигом `ETL_*`, Dockerfile и docker-compose-сервисом, Makefile-таргетами. После этой фазы `go build ./cmd/etl` собирается и `docker compose up etl` стартует без ошибок (с заглушкой Run).

## Commit

```
feat(etl): bootstrap cmd/etl binary + etlapp DI root + Dockerfile + compose
```

## Files to CREATE

- `cmd/etl/main.go` — entrypoint с graceful shutdown (SIGINT/SIGTERM), 30s shutdown timeout. Структура аналогична `cmd/source-adapter/main.go` (см. design-go-layers.md §1).
- `internal/etlapp/app.go` — `New(ctx, cfg, logger) (*App, error)`, метод `(a *App) Run(ctx)` (пока заглушка: log "etl app started, waiting"). DI bootstrap — pgxpool, JWT signer, http.Client (всё в виде stub-значений — реализуется в последующих фазах).
- `internal/etlapp/config/config.go` — envconfig prefix `ETL_*`. Поля: `DSN`, `HTTPPort` (default 8081), `SourceAdapterURL`, `JWTSigningKey`, `JWTRole` (default `x-flow-etl`), `AdminJWTSecret`, `CronSchedule` (default `30 2 * * *`), `CronTimezone` (default `Europe/Kyiv`), `RetryBackoffCap` (default 30s), `ValidationRulesPath` (default `./configs/etl_validation_rules.yaml`), `QualityThreshold` (default 0.01), `OTELExporterEndpoint`, `OTELTracesSamplerArg` (default 1.0), `LogLevel` (default `info`), `Env` (default `development`).
- `internal/etlapp/deps/deps.go` — `BuildDeps(ctx, cfg, logger) (*Deps, error)` — pgxpool init (через `config/db`), `httpClient`, `jwtSigner` (заглушка). Реальные интеграции — в последующих фазах.
- `Dockerfile.etl` — multi-stage (golang:1.26-alpine builder → scratch/distroless runtime). Bin path `/app/etl`. Expose 8081.
- `infrastructure/dev/docker-compose.yml` — добавить service `etl`: `image: e_zoo/etl:dev`, `build: { dockerfile: Dockerfile.etl }`, env-vars (см. список выше), `depends_on: [postgres, source-adapter]`, port `8081:8081`. (Уточнить точный путь compose-файла в зависимости от существующей инфры — если `infrastructure/dev/` отсутствует, использовать корневой `docker-compose.yml`.)
- `configs/etl_validation_rules.yaml` — пустой шаблон (заполняется в фазе 09):
  ```yaml
  version: 1
  rules: []
  ```

## Files to MODIFY

- `Makefile` — добавить таргеты:
  ```makefile
  ETL_DSN ?= postgres://e_zoo:e_zoo@localhost:5432/e_zoo?sslmode=disable
  ETL_MIG_PATH = internal/features/etl_validation/sqls/migrations

  build-etl:
  	go build -o bin/etl ./cmd/etl

  run-etl:
  	go run ./cmd/etl

  docker-build-etl:
  	docker build -f Dockerfile.etl -t e_zoo/etl:dev .

  migrate-up-etl:
  	migrate -database "$(ETL_DSN)" -path $(ETL_MIG_PATH) up

  migrate-down-etl:
  	migrate -database "$(ETL_DSN)" -path $(ETL_MIG_PATH) down 1

  migrate-version-etl:
  	migrate -database "$(ETL_DSN)" -path $(ETL_MIG_PATH) version

  test-integration-etl:
  	go test ./internal/features/etl_validation/... -tags=integration
  ```
- `go.mod` / `go.sum` — добавить зависимости: `github.com/go-co-op/gocron/v2` (для фазы 14, добавляем заранее, чтобы избежать дрейфа), `github.com/golang-jwt/jwt/v5` (если ещё не подключён). `go mod tidy`.
- `.env.example` — добавить новый блок `# --- ETL (Module 2) ---` со всеми `ETL_*` переменными.

## SQL / Migrations

Нет.

## Run after

```bash
go mod tidy
go build ./cmd/etl
go vet ./cmd/etl ./internal/etlapp/...
golangci-lint run ./cmd/etl/... ./internal/etlapp/...
docker build -f Dockerfile.etl -t e_zoo/etl:dev .
```

## Tests

- Smoke: `go run ./cmd/etl` стартует, ловит SIGINT, корректно завершается за <30s.
- (Без unit-тестов — это caркас. Тесты для `etlapp.New` появятся в фазе 13/15.)

## Definition of Done

- [ ] `cmd/etl/main.go` существует, graceful shutdown реализован.
- [ ] `internal/etlapp/app.go` экспортирует `New(ctx, cfg, logger) (*App, error)` и `Run(ctx) error`.
- [ ] `internal/etlapp/config/config.go` — все поля `ETL_*` загружаются через envconfig.
- [ ] `Dockerfile.etl` собирается, image работает.
- [ ] `docker-compose` сервис `etl` стартует и не падает в crashloop.
- [ ] Makefile-таргеты `build-etl`, `migrate-up-etl`, `migrate-down-etl`, `docker-build-etl`, `test-integration-etl` присутствуют.
- [ ] `go build ./...` проходит без ошибок.
- [ ] `golangci-lint run ./...` без новых нарушений.
- [ ] `.env.example` обновлён.

## Зависимости

Требует Модуль 1 (source-adapter) уже работает в проекте.
