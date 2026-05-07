# Phase 01: Bootstrap

**Цель:** поднять с нуля Go-проект `github.com/Kitavrus/e_zoo`: модуль, бинарь, конфиг, логгер, ошибки, docker-compose, Dockerfile, Makefile. На выходе — `docker-compose up postgres` поднимает PG18, `make build` собирает пустой бинарь, который стартует, отдаёт `/healthz` (минимум), gracefully завершается на SIGTERM.

**Commit:** `chore(bootstrap): инициализация модуля github.com/Kitavrus/e_zoo + docker-compose + Makefile`

---

## Files to CREATE

- `go.mod` — `module github.com/Kitavrus/e_zoo`, `go 1.26`. Зависимости: `github.com/gofiber/fiber/v3`, `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/kelseyhightower/envconfig`, `github.com/golang-migrate/migrate/v4`, `github.com/google/uuid`.
- `go.sum` — авто.
- `cmd/source-adapter/main.go` — entrypoint: load env через envconfig → `app.New(cfg)` → `app.Run(ctx)`. Graceful shutdown по `signal.NotifyContext(SIGINT, SIGTERM)`, `app.Shutdown(ctx)` с timeout 30s.
- `internal/app/app.go` — структура `App`: `cfg`, `pool *pgxpool.Pool`, `logger *slog.Logger`, `fiber *fiber.App`. Методы `New`, `Run`, `Shutdown`. Пока без DI деталей (заполнится в фазе 15). Только Fiber-конфиг (Prefork=false, ReadTimeout=30s, WriteTimeout=60s, BodyLimit=10MB) + единственный `/healthz` (200 OK, JSON `{"status":"ok"}`).
- `internal/config/config.go` — `type Config struct{}` через `envconfig.Process("", &cfg)`. Поля: `DBDsn`, `HTTPAddr`, `LogLevel`, `JwtAlg`, `JwtSecret`, `JwtPublicKeyPath`, `CronSchedule`, `CronTZ`, `ExportsDir`, `ExportsRetentionH` и т.д. Default-теги по [design-infrastructure.md](design-infrastructure.md) §«ENV vars».
- `internal/logger/logger.go` — `New(level string) *slog.Logger` (JSONHandler, level из env).
- `pkg/errorspkg/errors.go` — `type Error struct{ Code, Message, SupportMessage string; HTTP int; Wrapped error }`. Метод `Error()`. Конструкторы: `NewBadRequest`, `NewNotFound`, `NewConflict`, `NewUnauthorized`, `NewForbidden`, `NewServiceUnavailable`, `NewInternal`. Sentinel-ошибки фазы: `ErrBadRequest`, `ErrNotFound`, `ErrInternal`. Остальные sentinel добавляются в своих фазах.
- `pkg/errorspkg/response.go` — `type ErrorResponseJSON struct{ Code, Message, SupportMessage, TraceID string; Details []Detail }`. Хелпер `WriteJSON(c fiber.Ctx, err error)` — мапит наш `*Error` → ResponseJSON, иначе 500 internal.
- `pkg/errorspkg/errors_test.go` — unit (3+ кейса: ErrBadRequest, ErrNotFound, обёртка `errors.Is`).
- `Dockerfile` — multi-stage: `golang:1.26-alpine` builder → `alpine:3.20` runtime, non-root user `app`, `COPY --from=builder /app/bin/source-adapter /usr/local/bin/`, `EXPOSE 8080`, `ENTRYPOINT ["source-adapter"]`.
- `docker-compose.yml` — `postgres:18-alpine` (POSTGRES_USER=adapter, POSTGRES_PASSWORD=adapter, POSTGRES_DB=source_adapter, healthcheck pg_isready), `migrate` (profile `migrate`), `source-adapter` (depends_on healthy postgres). Точно как в [design-infrastructure.md](design-infrastructure.md) §1.
- `infra/pg/postgresql.conf` — минимальный tuning для PG18 (max_connections=100, shared_buffers=256MB, work_mem=8MB).
- `Makefile` — таргеты:
  - `build` → `go build -o bin/source-adapter ./cmd/source-adapter`
  - `run` → `go run ./cmd/source-adapter`
  - `test-unit` → `go test ./internal/... ./pkg/... -short -race`
  - `test-integration` → `go test ./internal/... ./test/e2e/... -race -tags=integration`
  - `test-all` → зависит от обоих
  - `lint` → `golangci-lint run --timeout 5m`
  - `migrate-up` / `migrate-down` / `migrate-create NAME=xxx` (golang-migrate CLI через docker run)
  - `docker-up` / `docker-down`
- `.env.example` — все ENV-переменные из `internal/config/config.go` с дефолтами.
- `.gitignore` — расширить: `bin/`, `coverage.out`, `var/exports/`.
- `.golangci.yml` — staticcheck, govet, errcheck, gofmt, gosimple, ineffassign, gosec.
- `README.md` — короткий «as-is» (имя модуля, как поднять локально через `make docker-up`, `make migrate-up`, `make run`). Не более 30 строк.

## Files to MODIFY

— нет (greenfield).

## SQL/Migrations

— нет.

## Run after

```bash
go mod tidy
make build
docker-compose up -d postgres
docker-compose exec postgres pg_isready -U adapter -d source_adapter
make run &
curl -s http://localhost:8080/healthz | jq .
```

## Tests in this phase

- `pkg/errorspkg/errors_test.go::TestErrBadRequestWrap`
- `pkg/errorspkg/errors_test.go::TestErrNotFoundIs`
- `pkg/errorspkg/errors_test.go::TestErrorResponseJSONShape`

## Definition of Done

- [ ] `go.mod` создан с модулем `github.com/Kitavrus/e_zoo`, `go 1.26`.
- [ ] `make build` собирает бинарь без ошибок.
- [ ] `docker-compose up -d postgres` стартует PG18, healthcheck зелёный за <30s.
- [ ] Сервис стартует, отдаёт `GET /healthz` 200, gracefully завершается на SIGTERM (<30s).
- [ ] `make test-unit` зелёный (3 теста errorspkg).
- [ ] `make lint` без ошибок.
- [ ] `.env.example` содержит все переменные из `internal/config/config.go`.
- [ ] Коммит атомарный, сообщение `chore(bootstrap): ...`.
