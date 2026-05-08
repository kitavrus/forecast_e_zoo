.PHONY: build build-all build-source-adapter build-etl build-data-marts build-kpi \
        build-forecast build-order-builder build-channel-router \
        run-source-adapter run-etl run-data-marts run-kpi \
        run-forecast run-order-builder run-channel-router \
        test test-unit test-integration test-all lint vet \
        compose-up compose-down compose-build compose-logs compose-ps \
        migrate-up-all \
        migrate-up-data-export migrate-down-data-export \
        migrate-up-etl       migrate-down-etl \
        migrate-up-kpi       migrate-down-kpi \
        migrate-up-forecast  migrate-down-forecast \
        migrate-up-orders    migrate-down-orders \
        migrate-up-channels  migrate-down-channels \
        migrate-create \
        test-integration-etl test-integration-kpi \
        e2e-up e2e-down e2e-full seed-channel-configs

# --- Сборка ---

BIN := bin

build: build-all

build-all: build-source-adapter build-etl build-data-marts build-kpi \
           build-forecast build-order-builder build-channel-router

build-source-adapter:
	go build -o $(BIN)/source-adapter  ./cmd/source-adapter
build-etl:
	go build -o $(BIN)/etl             ./cmd/etl
build-data-marts:
	go build -o $(BIN)/data-marts      ./cmd/data-marts
build-kpi:
	go build -o $(BIN)/kpi             ./cmd/kpi
build-forecast:
	go build -o $(BIN)/forecast        ./cmd/forecast
build-order-builder:
	go build -o $(BIN)/order-builder   ./cmd/order-builder
build-channel-router:
	go build -o $(BIN)/channel-router  ./cmd/channel-router

# --- Локальный запуск (без Docker) ---

run-source-adapter:
	go run ./cmd/source-adapter
run-etl:
	go run ./cmd/etl
run-data-marts:
	go run ./cmd/data-marts
run-kpi:
	go run ./cmd/kpi
run-forecast:
	go run ./cmd/forecast
run-order-builder:
	go run ./cmd/order-builder
run-channel-router:
	go run ./cmd/channel-router

# --- Тесты ---

test-unit:
	go test ./internal/... ./pkg/... -short -race

test-integration:
	go test ./internal/... ./test/e2e/... -race -tags=integration

test-all: test-unit test-integration
test: test-unit

test-integration-etl:
	go test ./internal/features/etl_validation/... -tags=integration -race

test-integration-kpi:
	go test ./internal/features/kpi/... -tags=integration -race

# --- Линт / vet ---

lint:
	golangci-lint run --timeout 5m
vet:
	go vet ./...

# --- Docker compose (вся инфра) ---

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down -v

compose-build:
	docker compose build

# Использование: make compose-logs SERVICE=source-adapter
compose-logs:
ifndef SERVICE
	docker compose logs --tail=200 -f
else
	docker compose logs --tail=200 -f $(SERVICE)
endif

compose-ps:
	docker compose ps

# --- Миграции через docker (golang-migrate CLI) ---
# Используется для локального запуска БЕЗ docker-compose (например, postgres
# поднят локально). В docker-compose всё это уже выполняется сервисом `migrate`.

DB_DSN ?= postgres://adapter:adapter@localhost:5432/source_adapter?sslmode=disable

DATA_EXPORT_MIGRATIONS := $(CURDIR)/internal/features/data_export/sqls/migrations
ETL_MIGRATIONS         := $(CURDIR)/internal/features/etl_validation/sqls/migrations
KPI_MIGRATIONS         := $(CURDIR)/internal/features/kpi/sqls/migrations
FORECAST_MIGRATIONS    := $(CURDIR)/internal/features/forecast/sqls/migrations
ORDERS_MIGRATIONS      := $(CURDIR)/internal/features/orders/sqls/migrations
CHANNELS_MIGRATIONS    := $(CURDIR)/internal/features/channels/sqls/migrations

define MIGRATE_RUN
docker run --rm --network host \
  -v $(1):/migrations \
  migrate/migrate:v4.18.1 \
  -path=/migrations -database "$(DB_DSN)" $(2)
endef

migrate-up-data-export:
	$(call MIGRATE_RUN,$(DATA_EXPORT_MIGRATIONS),up)
migrate-down-data-export:
	$(call MIGRATE_RUN,$(DATA_EXPORT_MIGRATIONS),down 1)

migrate-up-etl:
	$(call MIGRATE_RUN,$(ETL_MIGRATIONS),up)
migrate-down-etl:
	$(call MIGRATE_RUN,$(ETL_MIGRATIONS),down 1)

migrate-up-kpi:
	$(call MIGRATE_RUN,$(KPI_MIGRATIONS),up)
migrate-down-kpi:
	$(call MIGRATE_RUN,$(KPI_MIGRATIONS),down 1)

migrate-up-forecast:
	$(call MIGRATE_RUN,$(FORECAST_MIGRATIONS),up)
migrate-down-forecast:
	$(call MIGRATE_RUN,$(FORECAST_MIGRATIONS),down 1)

migrate-up-orders:
	$(call MIGRATE_RUN,$(ORDERS_MIGRATIONS),up)
migrate-down-orders:
	$(call MIGRATE_RUN,$(ORDERS_MIGRATIONS),down 1)

migrate-up-channels:
	$(call MIGRATE_RUN,$(CHANNELS_MIGRATIONS),up)
migrate-down-channels:
	$(call MIGRATE_RUN,$(CHANNELS_MIGRATIONS),down 1)

# Прогоняет ВСЕ миграции в правильном порядке (для локального dev).
migrate-up-all: migrate-up-data-export migrate-up-etl migrate-up-kpi \
                migrate-up-forecast migrate-up-orders migrate-up-channels

# Создание новой миграции в указанной фиче.
# make migrate-create FEATURE=channels NAME=add_xxx
migrate-create:
ifndef FEATURE
	$(error Usage: make migrate-create FEATURE=<data_export|etl_validation|kpi|forecast|orders|channels> NAME=имя_миграции)
endif
ifndef NAME
	$(error Usage: make migrate-create FEATURE=<feature> NAME=имя_миграции)
endif
	docker run --rm \
	  -v $(CURDIR)/internal/features/$(FEATURE)/sqls/migrations:/migrations \
	  migrate/migrate:v4.18.1 \
	  create -ext sql -dir /migrations -seq $(NAME)

# --- E2E (mock-erp + seeded channel configs) ---

# Поднимает весь docker-compose стек (включая mock-erp), ждёт пока все сервисы
# не станут healthy, затем сидит supplier_channel_config (50 строк).
e2e-up:
	docker compose up -d --build
	@echo "Waiting for services to be healthy..."
	@i=0; \
	while [ $$i -lt 60 ]; do \
	  unhealthy=$$(docker compose ps --format '{{.Status}}' | grep -E 'starting|unhealthy' | wc -l | tr -d ' '); \
	  if [ "$$unhealthy" = "0" ]; then break; fi; \
	  sleep 5; i=$$((i+1)); \
	done
	@$(MAKE) seed-channel-configs

e2e-down:
	docker compose down -v

# Полный прогон pipeline: cleanup → up → seed → E2E test → отчёт
# Делает ВСЁ: создаёт тестовые данные в mock-erp (1000 products / 30 locations / 365 days / 50 suppliers
# по умолчанию из .env), прогоняет через 7 микросервисов, отправляет PO обратно в mock-erp.
# Длительность: ~1-3 мин на small scale (10/2/7/3), ~5-15 мин на realistic (100/5/90/15), ~30+ мин на full (1000/30/365/50).
# Override scale через env vars: SEED_PRODUCTS=100 SEED_LOCATIONS=5 SEED_DAYS=90 SEED_SUPPLIERS=15 make e2e-full
e2e-full:
	@echo "==> [1/4] Cleanup previous state (docker compose down -v)..."
	@docker compose down -v 2>&1 | tail -3 || true
	@echo "==> [2/4] Bringing up stack with mock-erp seed (это займёт время — мок-ERP сидит данные)..."
	@$(MAKE) e2e-up
	@echo "==> [3/4] Running E2E pipeline test (8 stages: source-adapter → ETL → KPI → forecast → orders → channel-router → mock-erp received)..."
	@bash tests/e2e/run.sh --skip-up
	@echo ""
	@echo "============================================================"
	@echo "✅ E2E pipeline complete!"
	@echo ""
	@echo "  📊 Live dashboards:  http://localhost:8091/"
	@echo "     • /m0 — Mock ERP (source data, received POs)"
	@echo "     • /m1 — Source Adapter (public.* tables)"
	@echo "     • /m2 — ETL (marts.*)"
	@echo "     • /m3 — Data Marts API"
	@echo "     • /m4 — KPI (OSA, OTIF, Stock Days)"
	@echo "     • /m5 — Forecast (forecasts + replenishment_plans)"
	@echo "     • /m6 — Order Builder (purchase_orders)"
	@echo "     • /m7 — Channel Router (send_attempts)"
	@echo ""
	@echo "  🛑 To stop & cleanup:  make e2e-down"
	@echo "============================================================"

# Применяет seed_channel_configs.sql внутрь контейнера postgres.
seed-channel-configs:
	@docker exec -i ezoo_pg psql -U $${POSTGRES_USER:-adapter} -d $${POSTGRES_DB:-source_adapter} \
	  < tests/e2e/seed_channel_configs.sql
	@echo "OK supplier_channel_config seeded (50 rows)"
