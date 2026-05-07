.PHONY: build run test test-unit test-integration test-all lint \
        migrate-up migrate-down migrate-create \
        docker-build docker-up docker-down \
        build-etl run-etl docker-build-etl \
        migrate-up-etl migrate-down-etl migrate-version-etl migrate-create-etl \
        test-integration-etl

# --- Сборка ---

build:
	go build -o bin/source-adapter ./cmd/source-adapter

run:
	go run ./cmd/source-adapter

build-etl:
	go build -o bin/etl ./cmd/etl

run-etl:
	go run ./cmd/etl

# --- Тесты ---

test-unit:
	go test ./internal/... ./pkg/... -short -race

test-integration:
	go test ./internal/... ./test/e2e/... -race -tags=integration

test-all: test-unit test-integration
test: test-unit

# --- Линт ---

lint:
	golangci-lint run --timeout 5m

# --- Миграции (через Docker — golang-migrate CLI) ---

MIGRATE_PATH := /Users/igorpotema/mycode/e_zoo/internal/features/data_export/sqls/migrations
DB_DSN ?= postgres://adapter:adapter@localhost:5432/source_adapter?sslmode=disable

migrate-up:
	docker run --rm --network host \
	  -v $(MIGRATE_PATH):/migrations \
	  migrate/migrate:v4.18.1 \
	  -path=/migrations -database "$(DB_DSN)" up

migrate-down:
	docker run --rm --network host \
	  -v $(MIGRATE_PATH):/migrations \
	  migrate/migrate:v4.18.1 \
	  -path=/migrations -database "$(DB_DSN)" down 1

migrate-create:
ifndef NAME
	$(error Usage: make migrate-create NAME=имя_миграции)
endif
	docker run --rm \
	  -v $(MIGRATE_PATH):/migrations \
	  migrate/migrate:v4.18.1 \
	  create -ext sql -dir /migrations -seq $(NAME)

# --- ETL миграции (Module 2) ---

ETL_MIGRATE_PATH := /Users/igorpotema/mycode/e_zoo/internal/features/etl_validation/sqls/migrations
ETL_DSN ?= postgres://adapter:adapter@localhost:5432/source_adapter?sslmode=disable

migrate-up-etl:
	docker run --rm --network host \
	  -v $(ETL_MIGRATE_PATH):/migrations \
	  migrate/migrate:v4.18.1 \
	  -path=/migrations -database "$(ETL_DSN)" up

migrate-down-etl:
	docker run --rm --network host \
	  -v $(ETL_MIGRATE_PATH):/migrations \
	  migrate/migrate:v4.18.1 \
	  -path=/migrations -database "$(ETL_DSN)" down 1

migrate-version-etl:
	docker run --rm --network host \
	  -v $(ETL_MIGRATE_PATH):/migrations \
	  migrate/migrate:v4.18.1 \
	  -path=/migrations -database "$(ETL_DSN)" version

migrate-create-etl:
ifndef NAME
	$(error Usage: make migrate-create-etl NAME=имя_миграции)
endif
	docker run --rm \
	  -v $(ETL_MIGRATE_PATH):/migrations \
	  migrate/migrate:v4.18.1 \
	  create -ext sql -dir /migrations -seq $(NAME)

test-integration-etl:
	go test ./internal/features/etl_validation/... -tags=integration -race

# --- Docker ---

docker-build:
	docker build -t source-adapter:dev .

docker-build-etl:
	docker build -f Dockerfile.etl -t e_zoo/etl:dev .

docker-up:
	docker compose up -d postgres

docker-down:
	docker compose down
