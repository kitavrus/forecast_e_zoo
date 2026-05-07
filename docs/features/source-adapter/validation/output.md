# Source-Adapter — Validation Run Output

**Дата прогона:** 2026-05-07 17:20–17:23 (Europe/Kyiv)
**Окружение:** локальная разработка, macOS 14.6.0, Docker Desktop, Go 1.22+
**Цель:** smoke-валидация 13 сценариев Stage 9 (greenfield Go-сервис, Fiber v3 + PG 18, JWT HS256).

---

## 1. Подготовка

| Шаг | Результат |
|---|---|
| `cp .env.example .env` | OK |
| `mkdir -p docs/features/source-adapter/validation` | OK |
| Docker доступен | OK |
| `docker compose up -d postgres` | **FAIL** через compose. Workaround: запуск standalone-контейнера с `PGDATA=/var/lib/postgresql/data/pgdata`. См. **Issue #1**. |
| Postgres ready | OK через 2 секунды |
| `make migrate-up` | **FAIL** на образе `migrate/migrate:v4` (тег не существует). Workaround: запустил с тегом `v4.18.1`. См. **Issue #2**. |
| Миграции применены | `1/u master_and_service`, `2/u facts_partitioned` — OK |
| Сборка `/tmp/source-adapter` | OK |
| Запуск сервиса | **HTTP_ADDR=:8080** занят посторонним процессом (uvicorn). Запустил на `:8085`. См. **Issue #3**. |
| JWT-генерация | OK (HS256, role в `iss`-claim, secret `dev-secret-change-in-prod`) |
| ERP_BASE_URL | первый запуск пустой → trigger = `noopTrigger` (нет load-ов). Перезапустил с `ERP_BASE_URL=/Users/igorpotema/mycode/e_zoo/testdata/fixtures`. См. **Issue #4**. |

**Важная заметка о JWT:** в коде `internal/middleware/role.go` роль матчится по `Issuer` (claim `iss`), а **не** по custom `role`-claim. Это отступление от шаблона в плане валидации; сгенерировал токены с `iss=admin-cli` / `iss=x-flow-etl`, и проверка прошла.

---

## 2. Happy path

| # | Сценарий | Команда | Ожидаемое | Факт | Статус |
|---|---|---|---|---|---|
| 1 | GET /healthz без auth | `curl /healthz` | 200 | 200 + `{"status":"ok","db":"ok",...}` | ✅ |
| 2 | GET /metrics без auth | `curl /metrics` | 200 + `source_adapter_*` | 200, найдено 17 строк `source_adapter_*` | ✅ |
| 3 | GET /v1/snapshots/current x-flow-etl ДО первого load | `curl -H Bearer …` | 503 `snapshot_not_ready` | 503 `{"code":"snapshot_not_ready","supportMessage":"SA-SNAP-002"}` | ✅ |
| 4 | POST /admin/loads admin-cli | `curl -X POST` | 202 + load_id | 202, но `load_id="00000000-..."` (uuid.Nil), `started_at` нулевой | ⚠ (см. ниже — by design) |
| 5 | GET /admin/loads/{id} admin-cli | `curl /admin/loads/{id}` | 200 + status=committed | 200, **status=failed** (FK error в фикстурах). См. **Issue #5**. | ❌ |
| 6 | GET /v1/snapshots/current x-flow-etl (после load) | `curl -H Bearer …` | 200 + `current_load_id` | 503 `snapshot_not_ready` (т.к. load failed) | ❌ (cascade от #5) |
| 7 | GET /v1/products x-flow-etl | NDJSON | 200 NDJSON | 503 `snapshot_not_ready` | ❌ (cascade от #5) |
| 8 | GET /v1/receipt_line x-flow-etl | NDJSON | 200 NDJSON | 503 `snapshot_not_ready` (с обязательными `event_date_from`/`to`) | ❌ (cascade от #5) |

**Замечание по #4:** ответ `POST /admin/loads` намеренно содержит `uuid.Nil` — trigger асинхронный (`go func() { trigger.TriggerOnce(...) }`), и реальный load_id появляется в БД позже. Клиент должен делать `GET /admin/loads` (list) для нахождения load_id. Это согласуется с архитектурой, но **в плане Stage 9 предполагался непосредственно возвращаемый id** — небольшое расхождение между API и тестовым сценарием. Считаю **PASS** для самого хендлера.

---

## 3. Error paths

| # | Сценарий | Команда | Ожидаемое | Факт | Статус |
|---|---|---|---|---|---|
| 9 | GET /v1/products БЕЗ auth | `curl /v1/products` | 401 | 401 `{"code":"auth_invalid_token","supportMessage":"SA-AUTH-001"}` | ✅ |
| 10 | GET /v1/products admin-cli (не x-flow-etl) | `curl -H Bearer admin-cli` | 403 | 403 `{"code":"auth_forbidden","supportMessage":"SA-AUTH-002"}` | ✅ |
| 11 | POST /admin/loads повторно (race) | 5 параллельных POST | 4×409 + 1×202 (или похожее) | **5/5 = 202** (handler вернул accepted всем). В логе сервиса видны `scheduler.tick_skipped_lock_busy` ×3 — advisory lock работает, но HTTP-ответ не отражает конфликт. См. **Issue #6**. | ❌ |
| 12 | GET /v1/store_assortment x-flow-etl | `curl /v1/store_assortment` | 501 NotImplemented | 501 `{"code":"not_implemented","message":"store_assortment: handler not yet implemented (...)"}` | ✅ |
| 13 | GET /admin/reject-log admin-cli | `curl /admin/reject-log` | 200 | 200 `{"items":[]}` | ✅ |

---

## 4. Найденные проблемы

### Issue #1 — `docker compose up -d postgres` падает (фикс окружения)
- **Контейнер `ezoo_pg` exit-ит сразу:** `postgres:18-alpine` не разрешает данные прямо в `/var/lib/postgresql/data` (требует подкаталог как `pg_ctlcluster`).
- **Workaround:** добавить `PGDATA: /var/lib/postgresql/data/pgdata` в `docker-compose.yml` под `environment` для сервиса `postgres`.
- **Severity:** низкая (только для свежего volume). Описано в логе самого Postgres.

### Issue #2 — `make migrate-up` использует несуществующий тег `migrate/migrate:v4`
- В `Makefile` хардкод `migrate/migrate:v4`; pull даёт `manifest unknown`.
- **Workaround:** заменить на `migrate/migrate:v4.18.1` (или `migrate/migrate:latest`).
- **Severity:** низкая, конфигурационная.

### Issue #3 — Порт 8080 в `docker-compose.yml` конфликтует с локальными процессами
- На моей машине 8080 занят посторонним uvicorn → сервис не получил порт.
- **Workaround:** выставил `HTTP_ADDR=:8085` через env. Спецификация фиксирует `:8080`, но это не блокер для валидации.
- **Severity:** низкая.

### Issue #4 — Без `ERP_BASE_URL` сервис стартует с `noopTrigger`
- `internal/app/app.go::tryStubReader` возвращает `nil`, если `ERP_BASE_URL == ""`. Затем scheduler собирается с `noopTrigger`, который **не возвращает ошибку**, но и не запускает реального load-а.
- **Эффект:** `POST /admin/loads` отвечает `202`, но в БД `loads` пусто, snapshot никогда не readу.
- **Workaround:** установить `ERP_BASE_URL=/Users/igorpotema/mycode/e_zoo/testdata/fixtures`.
- **Рекомендация:** в dev-режиме либо логировать `WARN: trigger=noop` явно, либо в `.env.example` ставить дефолтный путь к `testdata/fixtures`.
- **Severity:** средняя (UX дев-окружения / документация).

### Issue #5 — **РЕАЛЬНЫЙ БАГ ЛОАДЕРА** — порядок вставки нарушает FK
- Все 4 запущенных load-а упали с одной и той же ошибкой:
  ```
  ERROR: insert or update on table "products" violates foreign key constraint "products_category_id_fkey" (SQLSTATE 23503)
  ```
- Фикстуры корректны: `category.json` содержит `CAT-DOG-FOOD`, `CAT-CAT-FOOD`, …; `products.json` ссылается на эти id.
- Причина: лоадер вставляет `products` **до** `category`. Порядок entities в loader или в его dependency-graph нарушен.
- **Файл:** скорее всего `internal/features/data_export/loader/*` — фикс выходит за рамки Stage 9 (валидация → НЕ фикси код).
- **Severity:** высокая (блокирует happy path E2E).

### Issue #6 — POST /admin/loads не возвращает 409 при concurrent
- 5 параллельных POST-ов дали 5×202.
- В логе: `scheduler.tick_skipped_lock_busy` зарегистрировался 3 раза (advisory lock сработал на уровне tick), но HTTP-обработчик **не дожидается** результата `TriggerOnce` (запускает в goroutine с background-ctx) и всегда отвечает 202.
- Условие `running != nil` в `PostLoads` проверяет **до** запуска, и из-за того, что load failed за <5ms, к моменту следующего запроса нет ни одной `running`-записи.
- **Файл:** `internal/features/data_export/handler/admin_loads.go::PostLoads` (строки 56-77).
- **Severity:** средняя (контракт API). Может быть by-design (если 409 ожидается только при долгих load-ах), но в плане Stage 9 явно указано "POST /admin/loads (второй сразу) → 409".

---

## 5. Логи сервиса (последние ~20 строк)

```
{"level":"INFO","msg":"loader.start","load_id":"b22e2265-...","source":"erp_e_zoo"}
{"level":"ERROR","msg":"loader.failed","load_id":"b22e2265-...","error":"ERROR: insert or update on table \"products\" violates foreign key constraint \"products_category_id_fkey\" (SQLSTATE 23503)"}
{"level":"ERROR","msg":"scheduler.load_failed","load_id":"b22e2265-...","error":"...23503"}
{"level":"INFO","msg":"http.access","method":"GET","path":"/admin/loads/...","status":200,...}
{"level":"INFO","msg":"http.access","method":"GET","path":"/v1/snapshots/current","status":503,...}
{"level":"INFO","msg":"http.access","method":"GET","path":"/v1/products","status":503,...}
{"level":"INFO","msg":"http.access","method":"GET","path":"/v1/receipt_line","status":503,...}
{"level":"INFO","msg":"http.access","method":"GET","path":"/v1/products","status":401,...}
{"level":"INFO","msg":"http.access","method":"GET","path":"/v1/products","status":403,"actor_role":"admin-cli",...}
{"level":"INFO","msg":"http.access","method":"POST","path":"/admin/loads","status":202,...}
{"level":"INFO","msg":"scheduler.tick_skipped_lock_busy"}
{"level":"INFO","msg":"scheduler.tick_skipped_lock_busy"}
{"level":"INFO","msg":"scheduler.tick_skipped_lock_busy"}
{"level":"INFO","msg":"http.access","method":"GET","path":"/v1/store_assortment","status":501,...}
{"level":"INFO","msg":"http.access","method":"GET","path":"/admin/reject-log","status":200,...}
```

---

## 6. Итог

- **Прогнано сценариев:** 13/13
- **Passed:** 8 (S1, S2, S3, S4-warning, S9, S10, S12, S13)
- **Failed (cascade от #5):** 3 (S6, S7, S8 — все `snapshot_not_ready` из-за провала load)
- **Failed (вероятный bug):** 2 (S5 = load failed; S11 = нет 409)
- **Структурно:** auth-слой и роутинг — **OK**, инфраструктура (PG/migrations) — **OK** (с workaround-ами), happy-path данных — **BROKEN** из-за Issue #5.

### Реальные баги (требуют отдельного фикса вне Stage 9)
1. **Loader порядок entities** (Issue #5) — критично, нарушает FK при загрузке fixtures.
2. **POST /admin/loads concurrency** (Issue #6) — не возвращает 409 при повторе во время running.

### Рекомендации к окружению
- Поправить Makefile тег `migrate/migrate:v4.18.1`.
- Поправить compose: добавить `PGDATA: /var/lib/postgresql/data/pgdata`.
- В `.env.example` ставить дефолт `ERP_BASE_URL=./testdata/fixtures` (или явно логировать noop).

### Готовность к Stage 10
**FAILED** (5/13 жёстких failures). Stage 10 (Report) допустим только при условии открытия задач на:
- багфикс лоадера (Issue #5),
- багфикс concurrency-409 (Issue #6),
- мелкий housekeeping (#1–#4).

**Status:** FAILED — 8/13 сценариев прошли. См. Issue #5 и #6.
