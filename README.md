# source-adapter

Модуль 1 MVP «Адаптер источников» проекта **E-Zoo**.

Module path: `github.com/Kitavrus/e_zoo`. Go 1.26.

## Локальный запуск

```bash
# 1. Скопировать env
cp .env.example .env

# 2. Поднять Postgres 18
make docker-up

# 3. Применить миграции (после фазы 03)
make migrate-up

# 4. Запустить адаптер локально
make run
```

После старта `GET /healthz` возвращает 200.

## Тесты

```bash
make test-unit          # быстрые юнит-тесты
make test-integration   # требует Docker (dockertest postgres:18-alpine)
```

## Подробная документация

См. `docs/features/source-adapter/` — design, code-plan, ADR, swimlane.
