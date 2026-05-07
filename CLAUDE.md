# CLAUDE.md — Project Configuration

---

## 1. Стек и архитектура

### Backend
- **Язык:** Go 1.26
- **Фреймворк:** Fiber v3 (`github.com/gofiber/fiber/v3`)
- **База данных:** PostgreSQL 18
- **DB driver:** pgx/v5 (`github.com/jackc/pgx/v5`, `pgxpool`)
- **SQL:** go:embed + pgx/v5 (SQL в .sql файлах, загрузка через `//go:embed`)
- **Архитектура:** Feature-based — `internal/features/{name}/handler|service|repository|models|router|sqls|validators` + `pkg/`
- **Миграции:** golang-migrate/v4, БЕЗ auto-apply (только `make migrate-up`), файлы в `migrations/`

### Frontend
- **Vue 3 + TypeScript** (только `<script setup>`, строгий режим)
- **State:** Pinia
- **Build:** Vite
- **Структура:** Feature-based (`frontend/src/features/{name}/api|store|composables|components`)

### Тестирование
- **Unit (Go):** go test + testify/suite
- **Integration (Go):** `github.com/ory/dockertest/v3` — реальный PostgreSQL 18 в Docker
- **Unit (Vue):** Vitest + Vue Test Utils
- **E2E (Web):** Chrome MCP (`mcp__claude-in-chrome__*`)
- **E2E (Backend):** Bash (curl, httpie, go test)

### Линтинг Backend — `.golangci.yml` (STRICT, 32 линтера)
```yaml
linters:
  enable:
    # Корректность
    - errcheck
    - staticcheck
    - govet
    - gosimple
    - ineffassign
    - unused
    - wrapcheck      # все ошибки обёрнуты через fmt.Errorf — обязательное правило
    - noctx          # нет context.Background() внутри handlers/service
    - nilerr
    - bodyclose
    - sqlclosecheck  # pgx rows/statements closed
    - rowserrcheck   # pgx rows.Err() проверен
    - contextcheck
    # Безопасность
    - gosec
    # Стиль
    - gofmt
    - goimports
    - gci            # группировка импортов: std / external / internal
    - revive
    - exhaustive
    - gocritic
    - nakedret
    - whitespace
    - godot
    - misspell
    - dupword
    - gofumpt
    # Сложность
    - cyclop
    - funlen
    - gocognit
    - nestif
    - maintidx
    # Производительность
    - prealloc
    # Extra
    - mnd
    - unparam

linters-settings:
  cyclop:
    max-complexity: 10
  funlen:
    lines: 60
    statements: 40
  gocognit:
    min-complexity: 15
  nestif:
    min-complexity: 4
  maintidx:
    under: 20
  mnd:
    checks: [argument, case, condition, return]
    ignored-numbers: ['0', '1', '2']
  gci:
    sections:
      - standard
      - default
      - prefix(your-module)   # ← заменить на имя Go-модуля из go.mod
  revive:
    rules:
      - name: exported
      - name: var-naming
      - name: error-return
      - name: error-naming
      - name: if-return
      - name: increment-decrement
      - name: var-declaration
      - name: package-comments
      - name: range
      - name: receiver-naming
      - name: time-naming
      - name: unexported-return
      - name: indent-error-flow
      - name: errorf
  gosec:
    excludes: [G401, G501]
  gocritic:
    enabled-tags: [diagnostic, style, performance]
  godot:
    scope: declarations
    capital: false

run:
  timeout: 5m
```

### Линтинг Frontend — `frontend/eslint.config.ts` (strict)
```typescript
import pluginVue from 'eslint-plugin-vue'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'

export default defineConfigWithVueTs(
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.strict,
  {
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/explicit-function-return-type': ['error', { allowExpressions: true }],
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/consistent-type-imports': 'error',
      'vue/no-unused-vars': 'error',
      'vue/no-unused-components': 'error',
      'vue/component-name-in-template-casing': ['error', 'PascalCase'],
      'no-console': 'error',
      'no-debugger': 'error',
    },
  },
)
```
> `//nolint` и `eslint-disable` без комментария-обоснования — **блокер при ревью**.

### API
- **REST + OpenAPI 3.0** — контракт является источником правды
- Фронт и бэк строго следуют спецификации

### Структура Go проекта
```
cmd/server/main.go
config/
├── db/                             ← package dbconfig (NewPool, pgxpool)
├── app/                            ← package appconfig (Config struct + Load())
└── firebase/                       ← package firebaseconfig (FCM клиент)
migrations/                         ← SQL файлы (golang-migrate, БЕЗ auto-apply)
internal/
├── app.go                          ← инициализация Fiber + middleware
├── routers/routers.go              ← регистрация всех фич
├── middleware/                     ← JWT, Admin и прочие middleware
└── features/
    └── {name}/
        ├── handler/                ← Fiber v3 handlers (один файл на action)
        │   ├── handler.go          ← struct + NewHandler() конструктор
        │   └── {action}.go         ← один action = один файл (snake_case: create.go, get_by_id.go)
        ├── mappers/                ← package mappers (error mapping: service errors → HTTP responses)
        │   └── helpers.go          ← MapServiceError(), Map{Action}Error()
        ├── models/                 ← package models (DB row types / domain)
        │   └── dto/                ← package dto (Request/Response DTOs)
        ├── repository/             ← pgx/v5 + go:embed
        ├── router/                 ← /api/v1/{name}/*
        ├── service/                ← бизнес-логика
        ├── sqls/                   ← .sql файлы (загружаются через go:embed)
        ├── validators/             ← валидация запросов
        └── constants/              ← package constants (feature-specific)
pkg/
├── errorspkg/                      ← sentinel errors + коды ошибок
└── utils/                          ← вспомогательные функции
```

### Ключевые правила архитектуры
- Нет бизнес-логики в handlers (только bind → service → response)
- Нет SQL в service слое (только в repository через go:embed + pgx)
- Общая инфраструктура только в `pkg/` (не дублируется между фичами)
- **`pkg/` — ТОЛЬКО для кода, используемого 2+ фичами.** Если функция/тип используется только в одной фиче — она живёт внутри этой фичи (в service/, validators/, или отдельном файле), НЕ в `pkg/utils/`, `pkg/support/` и т.д.
- Используется `pgxpool.Pool`, не `sql.DB`; подключение через `config/db` (package dbconfig)
- Все queries через go:embed .sql файлы в sqls/ (не raw строки в Go коде)
- Миграции через `make migrate-up`, НЕ auto-apply при старте; файлы в `migrations/`
- models/ — domain types (package models); models/dto/ — Request/Response (package dto)
- constants/ — feature-specific константы (package constants), при импорте нескольких фич — алиас
- handler/ — один файл на action: `handler.go` (struct + New), `{action}.go` (метод)
- mappers/ — error mapping в отдельном пакете: `MapServiceError()`, `Map{Action}Error()` (НЕ в handler/helpers.go)

### Fiber v3 обязательный синтаксис
```go
// handler.go — struct + конструктор (ТОЛЬКО это, без action-методов)
type Handler struct {
    svc service.Service
}
func NewHandler(svc service.Service) *Handler {
    return &Handler{svc: svc}
}

// create.go — ОДИН action = ОДИН файл (snake_case имя файла)
func (h *Handler) Create(c fiber.Ctx) error { ... }

// Bind запроса — v3 синтаксис
c.Bind().JSON(&dto)          // НЕ c.BodyParser()

// mappers/helpers.go — error mapping (MapServiceError, Map{Action}Error)

// Роуты (router/router.go)
g := router.Group("/users")
g.Post("/", h.Create)
```

### Integration тест — шаблон (dockertest/v3)
```go
//go:build integration

// Образ: postgres:18-alpine (всегда)
// Suite: testify/suite
// SetupTest: TRUNCATE TABLE ... CASCADE (изоляция)
// TearDownSuite: db.Close() + pool.Purge(resource)
// AutoRemove: true, MaxWait: 30s
```

### Handler integration тест — шаблон (dockertest/v3 + Fiber app.Test())
```go
//go:build integration

// Расположение: internal/features/{name}/handler/{name}_handler_integration_test.go
// Образ: postgres:18-alpine (всегда)
// Suite: testify/suite
// DI: repository.New(db) → service.New(repo) → handler.NewHandler(svc, validator)
// App: fiber.New() + router.Register(app.Group("/api"), h, middlewares...)
// SetupTest: TRUNCATE TABLE ... CASCADE (изоляция)
// TearDownSuite: db.Close() + pool.Purge(resource)
// AutoRemove: true, MaxWait: 30s
// Coverage: ≥90% пакета handler/

// Обязательные сценарии для каждого action:
// - Happy path → ожидаемый HTTP status + структура ответа
// - Невалидный JSON → 400
// - Провал валидации → 400
// - Not found → 404
// - Дублирование → 409 (если применимо)
// - Protected route без JWT → 401
// - Protected route с чужим ресурсом → 403 (если применимо)

// JWT helper для protected routes:
// func generateTestJWT(userID, secret string) string { ... }
```

---

## 2. Quality Gates (обязательно перед коммитом)

```bash
# Go
go build ./...
golangci-lint run ./...
go test ./... -race -count=1
go test -tags=integration ./... -count=1   # требует Docker
go vet ./...

# Handler coverage (integration) — ОБЯЗАТЕЛЬНО ≥90%
go test -tags=integration -coverprofile=handler_coverage.out \
    ./internal/features/*/handler/...
go tool cover -func=handler_coverage.out | grep total
# Ожидается: total: (statements) ≥ 90.0%

# Миграции
make migrate-up   # файлы в migrations/ (не pkg/db/migrations/)

# Vue
npm run type-check
npm run lint
npm run test -- --run
```

---

## 3. Принципы работы (STRICT)

- **Имею право не соглашаться** с решениями пользователя. Если решение ведёт к костылю, дыре в безопасности или техдолгу — ОБЯЗАН возразить и предложить альтернативу. Молчаливое согласие с плохим решением = ошибка.
- **Качество и security > скорость.** Не принимать "потом поправим", "сойдёт для MVP", "это временно". Временные решения становятся постоянными.
- **Долгосрочная польза > быстрый результат.** Выбирать решения, которые масштабируются и поддерживаются.
- Если пользователь настаивает на костыльном решении — чётко обозначить риски и зафиксировать в Report.

---

## 4. Выбор профиля (STRICT)

Каждый запрос обрабатывается в рамках ОДНОГО профиля. Профиль определяется **комбинированно**:

1. **Автодетект** по ключевым словам:
   - Баг, ошибка, краш, не работает, ломается, исключение, stacktrace, 500, regression → **Поиск бага**
   - Фича, добавить, реализовать, новый экран, интеграция, API endpoint → **Бизнес-фича**
   - Рефакторинг, рефакторить, мигрировать, переписать, upgrade, апгрейд, перейти с X на Y, migrate, legacy, технический долг → **Миграция/Рефакторинг**
   - инфра, деплой, провижн, nginx, ansible, docker-compose, prometheus, grafana, patroni, wal-g, replication, ssl, мониторинг, alert rule, alertmanager → **Инфра-изменение** → использовать `/infra-pipeline`
2. **Подтверждение** через `AskUserQuestion`: "Определил профиль: **<название>**. Верно?"
3. Явное указание профиля пользователем — подтверждение не требуется.

### Доступные профили

| Профиль | Когда использовать |
|---|---|
| Бизнес-фича | Новая функциональность, доработка, интеграция |
| Поиск бага | Баг, регрессия, краш, неожиданное поведение |
| Миграция/Рефакторинг | Рефакторинг, переписывание, upgrade зависимостей, миграция БД |
| **Инфра-изменение** | Docker, Ansible, Nginx, Prometheus, Patroni, CI/CD, SSL — использовать `/infra-pipeline` |

---

## 4.5. Triage & Tier Selection (STRICT — применяется ко всем профилям)

Сразу после выбора профиля оркестратор проводит **Triage** — короткое профилирование задачи, которое определяет tier (S/M/L) и набор флагов для Artifact Registry. Все стадии workflow для всех профилей остаются, но набор обязательных артефактов (секции, файлы, диаграммы, число research sub-agents, размер чеклиста ревью) масштабируется по tier.

### Шаг 1. Triage (оркестратор, ≤60 секунд, без отдельного агента)

Оркестратор заполняет ярлык и флаги:

```yaml
tier: S | M | L
touches:
  db: true|false        # изменяются миграции/SQL/схема
  fe: true|false        # затрагивается frontend/
  infra: true|false     # docker/ansible/nginx/prometheus/k8s
  external: true|false  # новые внешние HTTP/gRPC/webhook/integrations
risk: reversible | irreversible | data-migration
novelty: standard-crud | new-pattern | new-integration
decisions: []           # реальные развилки с ≥2 вариантами и trade-off
```

Источники: запрос пользователя + быстрый Grep по коду (найти, существует ли прецедент паттерна). Принцип заполнения `decisions[]` — **только реальные** trade-off; «почему не делать X» записывается одной строкой, не как развилка.

**`risk` оценивается по user-facing impact, а НЕ по ревертабельности git-коммита:**
- `reversible` — фича/фикс можно откатить без последствий для пользователей и внешних интеграций (изменение внутреннего модуля, рефакторинг без API-влияния, dual-prefix переходный период)
- `irreversible` — после деплоя внешние клиенты/мобильные/интеграции получают breaking behavior (hard cutover URL, смена формата ответа, удаление endpoint) — **даже если git revert технически возможен**
- `data-migration` — миграции БД с backfill/изменением данных, где rollback затратен или требует восстановления из бэкапа

Пример: миграция `/api/*` → `/api/v1/*` с hard cutover = **irreversible**, хотя код ревертабелен — мобильные клиенты получат 404.

### Шаг 2. Определение tier

**Правило:** tier = max по всем признакам. Один L-признак → весь профиль L.

| Признак | S | M | L |
|---|---|---|---|
| Endpoints добавлено/изменено | 0–1 | 2–4 | 5+ |
| Новых DB сущностей | 0 | 1–2 | 3+ |
| Миграций (additive/data) | 0 или 1 additive | 1–2 additive | 2+ или data-migration/backfill |
| Внешние интеграции | нет | 1 | 2+ |
| Затронутые фичи | 1 | 1–2 | cross-feature |
| Breaking changes API | нет | opt-in | да |
| Ожидаемый diff | <400 LOC | 400–1500 | >1500 |

Пограничные случаи (между S/M или M/L) — `AskUserQuestion` с обоснованием и предложенным tier.

### Шаг 3. Artifact Registry — таблица триггеров

Артефакты включаются в output ТОЛЬКО при выполнении условия. Независимо от tier:

| Артефакт | Условие включения |
|---|---|
| **PRD (Product Requirements)** | Бизнес-фича M/L tier (всегда); миграция L tier с user-facing impact; для S — inline 3 строки в `spec-interview/output.md`. **Не создаётся для bug-профиля и infra-профиля.** |
| C4 Context | `touches.external=true` ИЛИ затронуто ≥2 фич |
| C4 Container | новый контейнер (redis/queue/worker/sidecar) |
| Sequence diagram | ≥2 async hops ИЛИ ≥2 сервиса |
| go-layers секция | `novelty=new-pattern` (отход от handler→service→repo) |
| ADR запись | элемент в `decisions[]` с реальным trade-off (≥2 вариантов) |
| SQL/миграции секция | `touches.db=true` И (новая миграция ИЛИ >1 запрос) |
| Errors секция | ≥1 новый код `supportMessage` |
| Validators секция | non-trivial валидация (не просто `required`) |
| Tests-plan секция | `risk=irreversible` ИЛИ затронуто ≥2 фич |
| Contract секция | всегда если есть HTTP-поверхность |
| Repo секция | `touches.db=true` |
| Rollback секция | миграции ИЛИ `risk ∈ {irreversible, data-migration}` |
| Swimlane диаграмма | миграция L-tier с несколькими ролями или фазами ≥5 |
| **Monitoring & Alerting Spec** | новая фича добавляет spans/metrics/log fields ИЛИ затрагивает hot path ИЛИ `risk=irreversible`. Содержит таблицу: span name → log fields → metric → alert threshold. |
| **Test Strategy (расширенный)** | бизнес-критичная логика (auth, applications, payments-like) ИЛИ ≥2 ролей пользователей ИЛИ финансовые/контрактные операции. Таблица: сценарий → тип теста → fixtures → критерии успеха. |
| **Performance Budget** | endpoint в hot path (vacancies listing/search/feed) ИЛИ `novelty=new-pattern` для запроса с fan-out ИЛИ `touches.external=true` с латентностью. Таблица: endpoint → p50/p95/p99 → RPS budget → load-test snippet (k6/vegeta) или explicit "deferred + issue link". |
| **Rollout Strategy** | `risk=irreversible` ИЛИ A/B сценарий ИЛИ ручное opt-in (env-var/appconfig flag). Описывает: флаг, ramp-up план (0% → 10% → 100% / dark launch), critical metrics для отката без redeploy. |
| **Runbook (post-deploy)** | M/L tier для бизнес-фичи; всегда для миграции с `risk=data-migration`; для infra покрывается отдельным агентом `infra-monitoring`. Содержит: шаги деплоя, smoke-проверки, дашборды для наблюдения 24ч (Grafana ссылки), что делать при алертах. |
| Любая Mermaid/swimlane-диаграмма (C4 / dataflow / sequence / swimlane / «До/После» / Network / CI / Secrets) | Флаг = `yes` в `docs/{features\|infra}/{name}/diagrams-prefs.md`. **Решение пользователя переопределяет все остальные триггеры в этой таблице** — `no` пропускает диаграмму даже на L-tier, `yes` рисует её даже на S-tier. Файл создаётся оркестратором (`pipeline.md` 3.5 / `infra-pipeline.md` ORCH-3) сразу после определения профиля и имени фичи. |

### Hard invariants (независимо от tier)

- **Security-секция в ревью** присутствует всегда.
- **Rollback-стратегия** присутствует для любой миграции и любой `risk=irreversible`.
- **ADR** обязателен для каждого элемента `decisions[]` с trade-off (не таблица «почему НЕ делать»).
- **PRD ↔ ADR linkage:** если PRD создан (M/L tier бизнес-фичи), каждое ADR в `design.md` обязано иметь поле `**Driver:** PRD §X.Y — <цитата business-цели>`. ADR без driver → блокер в `design-reviewer` и `reviewer`. Для S-tier (inline PRD) driver указывает на конкретную строку inline-PRD из `spec-interview/output.md`.
- **PRD Success Metrics обязательно измеримы.** "Юзеру стало удобнее" / "интерфейс приятнее" — НЕ метрика. Принимаются: бизнес-метрики (conversion >X%, retention, NPS, time-to-task, support-load), технические SLA (p95 latency <Xms, RPS, error rate <X%), операционные (снижение manual-work admin'а на X%). Минимум одна метрика обязана фигурировать в Test Strategy сценариях или быть наблюдаемой через Monitoring Spec.
- **PRD ↔ Tech Spec context flow:** Фаза 2 агента `spec-interview.md` обязана перечитать `prd/output.md` перед началом и НЕ дублировать вопросы по уже зафиксированным бизнес-целям; технические вопросы формулируются с явной ссылкой на пункты PRD.
- **Tests:** минимум 1 happy + 1 error path. **Для бизнес-критичных фич (Test Strategy расширенный triggered) — обязательно покрытие минимум 1 acceptance criterion из PRD §3 конкретным тестом.**
- **Breaking-change note** в Report при изменении API.
- **Swagger Regeneration** при правке Go swag-аннотаций (см. 6/7.5).
- **Текстовые секции дизайна (ADR, contract, repo, sql, errors, validators, tests-plan, rollback, blast radius, health-check)** создаются всегда — независимо от выбора диаграмм в `diagrams-prefs.md`. Пользователь может отключить визуализацию, но не текстовое описание.
- **Monitoring Spec без алертов** запрещён: если фича добавляет spans/metrics, обязан быть либо хотя бы 1 алерт с конкретным порогом, либо явное обоснование "алерт отложен до post-launch + issue link" в секции.
- **Performance Budget без load-test инструмента** запрещён на L-tier: либо k6/vegeta/wrk скрипт-snippet (даже минимальный), либо explicit "deferred + issue link". Wishful thinking ("ожидаем p95 ~500ms") — блокер.

### Шаг 4. Масштабирование стадий по tier

Матрица стадий × tier описана в каждом профиле (разделы 6, 7, 7.5, 7.6) в подсекции `### Tier-specific artifacts`. Общий принцип:

| Стадия | S | M | L |
|---|---|---|---|
| Research (sub-agents) | 1 (по `touches`) | 2–3 (по `touches`) | 5 (все) |
| Spec Interview | inline в research/output.md, ≤5 Q | отдельный файл, 5–10 Q | полный, 10+ Q |
| Design | 2–3 секции (один файл) | 5–7 секций + 1 Mermaid | все секции (как сейчас) |
| Plan | inline checklist в design/output.md | planner/output.md | planner/output.md + milestones |
| Review | Slim (5 секций) | Standard (10 секций) | Full (15 секций) |
| Validation | smoke + unit + build | + integration | + load/migration dry-run |
| Report | 1 абзац | стандарт | полный + retrospective |

### Шаг 5. Фиксация в output

Первый artefact стадии Research/Diagnose обязан содержать в шапке YAML-блок с результатами Triage — дальнейшие агенты читают его и масштабируют свою работу.

```yaml
# Triage
tier: S
touches: {db: true, fe: false, infra: false, external: false}
risk: reversible
novelty: standard-crud
decisions: []
```

### Legacy / откат

Задачи, начатые до внедрения Triage, могут фиксировать `legacy: true` в шапке и выполняться по старому workflow. Откат изменения — удалить раздел 4.5 и вернуть prompts агентов к предыдущей версии; tier-метки безвредны в legacy-режиме.

---

## 5. Общие правила (STRICT — применяются ко всем профилям)

### Субагенты по стадиям — общий принцип

Каждая стадия выполняется ОТДЕЛЬНЫМ субагентом через Task tool. Главный контекст — оркестратор. Оркестратор:
- управляет переходами между стадиями
- передаёт контекст между субагентами
- показывает пользователю краткие итоги каждой стадии
- НЕ выполняет работу стадий напрямую

### Передача контекста между стадиями

При запуске субагента в prompt ОБЯЗАТЕЛЬНО передавать:
1. Исходный запрос пользователя
2. Краткий итог предыдущей стадии
3. Если откат — причину отката

### Язык вывода агентов (STRICT)

Все результаты работы агентов (output-файлы, отчёты, спецификации, планы) ОБЯЗАТЕЛЬНО на русском языке. Это включает:
- Заголовки и текстовые описания
- Комментарии и пояснения
- Названия секций в Markdown
- Описания в Mermaid-диаграммах

Исключения (остаются на английском):
- Код (Go, SQL, TypeScript, Vue)
- Технические термины без устоявшегося перевода (middleware, handler, repository)
- Имена в Mermaid C4-диаграммах (Component, Container, System)

### Субагенты проекта

| Роль | Файл агента | Зона |
|---|---|---|
| Архитектор (Go) | `.claude/agents/research.md` | Research, архитектура |
| Спек-интервьюер | `.claude/agents/spec-interview.md` | Глубокое интервью, spec файл |
| Дизайнер | `.claude/agents/design.md` | C4, dataflow, ADR |
| Backend разработчик | `.claude/agents/backend-dev.md` | Go, Fiber, pgx, go:embed |
| Frontend разработчик | `.claude/agents/frontend-dev.md` | Vue 3, TypeScript, Pinia |
| Планировщик | `.claude/agents/planner.md` | code-plan.md |
| Ревьюер / Security | `.claude/agents/reviewer.md` | Code review, безопасность |
| Mobile API Ревьюер | `.claude/agents/mobile-api-reviewer.md` | API/Swagger глазами клиента (контракт, breaking changes, N+1, error codes). Параллельно с design/code review + на validation после deploy. |
| Bug Researcher | `.claude/agents/bug-researcher.md` | Диагностика багов (backend/frontend/infra) |
| Migration Designer | `.claude/agents/migration-design.md` | Архитектура миграций и рефакторинга |

### Validation — общий порядок

**Шаг 1:** `AskUserQuestion` — "На каких платформах тестируем?"
Варианты: Backend, Web

**Шаг 2:** Сформировать E2E сценарий и сохранить:
```
docs/features/{name}/validation/output.md
```

```markdown
# E2E Scenario: <название>
Платформы: Backend, Web

## Шаги
- [ ] 1. Открыть экран X
- [ ] 2. Нажать кнопку Y
- [ ] 3. Проверить что отображается Z
```

**Шаг 3:** Сборка и unit тесты через Bash субагент:
```bash
go build ./...
go test ./... -race
npm run test -- --run
```

**Шаг 4:** Проверки по платформам:

| Платформа | Инструмент |
|---|---|
| Web (Vue) | Chrome MCP (`mcp__claude-in-chrome__*`) |
| Backend (Go) | Bash (curl, httpie, go test, integration tests) |

**Шаг 5:** После каждого шага обновлять файл сценария:
```markdown
- [x] 1. Открыть экран X ✅ (проверено)
- [ ] 2. Нажать кнопку Y
```

**ВАЖНО — устойчивость к компактизации контекста:**
- Файл `docs/features/{name}/validation/output.md` — персистентное состояние валидации
- Перед каждым действием в Validation — ПЕРЕЧИТАТЬ файл через Read tool
- Выполненные шаги (`[x]`) — НЕ проверять повторно
- Продолжать с первого невыполненного шага (`[ ]`)

**Шаг 6:** Ошибки → откат с описанием. Файл сохраняется, невыполненные шаги остаются.

### Report — сохранение отчётов

```
docs/features/{name}/report/output.md
```

---

## 6. Профиль: Бизнес-фича

### Tier-specific artifacts (см. 4.5)

Все стадии ниже остаются обязательными. Объём артефактов масштабируется по tier:

| Стадия | S | M | L |
|---|---|---|---|
| Research | 1 sub-agent (по `touches`) | 2–3 sub-agents | 5 sub-agents |
| **Spec Interview (PRD-first)** | **PRD inline 3 строки в `spec-interview/output.md`; Tech Spec ≤5 Q (без блоков 9–10)** | **Фаза 1: `prd/output.md` (Блоки 0–3) + пауза. Фаза 2: `spec-interview/output.md` (Блоки 4–10), 5–10 Q, Блок 9 обязателен** | **Полный PRD с альтернативами и rejected-options. Tech Spec 10+ Q. Блоки 9–10 оба обязательны** |
| Design | 2–3 секции (contract + repo [+ sql]) в одном `design/output.md`; Monitoring inline 1 строка | 5–7 секций + 1 Mermaid + **Monitoring & Alerting Spec (таблица) + Test Strategy (таблица сценариев)** | все секции согласно Artifact Registry + **Runbook + Performance Budget + Rollout Strategy (если применимо)** |
| Plan | inline checklist в `design/output.md` | `planner/output.md` (включая Phase 11.5 если есть Monitoring Spec) | `planner/output.md` + milestones + Phase 11.5/11.6 при наличии Monitoring/Runbook |
| Executing | без изменений | без изменений | без изменений |
| Review | Slim (5 секций) | Standard (10) + секция "PRD linkage" | Full (15) + Sec 16 Operability + Sec 17 PRD ↔ ADR linkage |
| Validation | smoke + unit + build | + integration + Jaeger spans check (если Monitoring Spec) | + load/migration dry-run + PRD acceptance trace в smoke-сценариях |
| Report | 1 абзац | стандарт | полный + retrospective + business outcome vs PRD success metrics |

Hard invariants (security/rollback/ADR/tests/PRD↔ADR/Monitoring/Performance Budget) сохраняются на всех tier — см. 4.5.

### Инварианты стека (нельзя нарушать)

Выбор технологий фиксирован. Предлагать или использовать альтернативы ЗАПРЕЩЕНО.

| Слой | Технологии | Агент |
|---|---|---|
| Backend | Go 1.26, Fiber v3, PostgreSQL 18, pgx/v5, go:embed, dockertest/v3 | `backend-dev.md` |
| Frontend | Vue 3, TypeScript, Pinia, Vitest | `frontend-dev.md` |

### Development Workflow (STRICT)

#### Стадии
1. **Research** — исследование кодовой базы, зависимостей, контекста
2. **Spec Interview** — глубокое интервью с пользователем, запись спецификации
3. **Design** — архитектурное решение (C4, dataflow, sequence, ADR)
4. **Plan** — planner/output.md с фазами и коммитами
5. **Executing** — написание кода по плану
6. **Review** — code review реализации
7. **Validation** — тесты, сборка, E2E
8. **Report** — отчёт
9. **Done** — завершено

#### Разрешённые переходы
```
Research        → Spec Interview
Spec Interview  → Design
Spec Interview  → Research      (нужен дополнительный контекст из кода)
Design          → Plan
Design          → Research      (нужно уточнение)
Design          → Spec Interview (пользователь хочет уточнить требования)
Plan            → Executing
Executing       → Review
Executing       → Research      (обнаружено неизвестное)
Executing       → Design        (план не покрывает случай)
Review          → Validation
Review          → Executing     (найдены блокеры в reviewer/output.md)
Validation      → Report
Validation      → Executing     (нашли дефект)
Validation      → Design        (архитектурная проблема)
Report          → Done
```
Все остальные переходы ЗАПРЕЩЕНЫ. Перед сменой стадии — явно указывать текущую и следующую.

#### Субагенты по стадиям

| Стадия | Агент | Модель | Что делает |
|---|---|---|---|
| Research | Консилиум (↓) | opus | Параллельный анализ кодовой базы |
| Spec Interview | `spec-interview.md` | opus | Интервью с пользователем → spec.md |
| Design | `design.md` | opus | C4, dataflow, ADR |
| Plan | `planner.md` | opus | planner/output.md по фазам |
| Executing Backend | `backend-dev.md` | opus | Go код |
| Executing Frontend | `frontend-dev.md` | opus | Vue код |
| Review | `reviewer.md` | opus | Code review по 15-секционному чеклисту |
| Mobile API Review | `mobile-api-reviewer.md` | opus | API/Swagger глазами клиента (parallel с design/code review). Trigger: только если diff трогает HTTP-поверхность |
| Validation | Bash + Chrome MCP | sonnet | Сборка, тесты, E2E (+ mobile-api-reviewer на validation-стадии если есть live dev URL) |
| Report | general-purpose | haiku | Отчёт |

> Executing Backend и Frontend запускаются **параллельно** через Task tool.

#### Research — 5 Sub-agents (research.md)

Research agent запускает 5 sub-agents **параллельно**:

| Роль | Зона ответственности |
|---|---|
| Backend Researcher | Go код, интерфейсы, error flow, валидация, go.mod |
| Frontend Researcher | Vue features, Pinia, API composables (если есть frontend/) |
| Contract Researcher | Маршруты, DTO, env vars, rate limiting, error responses |
| Infrastructure Researcher | Docker, CI/CD, Dockerfile, мониторинг, логирование |
| Test Researcher | Unit/integration тесты, моки, CI test stages, coverage |

**Порядок работы:**
1. Оркестратор запускает всех агентов **параллельно** с описанием задачи
2. Собирает результаты, формирует сводное резюме консилиума
3. `AskUserQuestion` если нужно уточнение
4. Только после полного сбора контекста — переход на следующую стадию

Результат сохраняется в: `docs/features/{name}/research/output.md`

#### Spec Interview — спецификация требований

Агент `spec-interview.md` получает `research/output.md` и проводит глубокое интервью с пользователем через `AskUserQuestion` по 7 блокам: проблема и цель, пользовательские сценарии, технические ограничения и tradeoffs, безопасность и доступ, UI/UX, обработка ошибок, компромиссы.

Результат сохраняется в: `docs/features/{name}/spec-interview/output.md`

Файл содержит: happy path, edge cases, матрицу ошибок, принятые компромиссы, отклонённые решения и открытые вопросы для Design агента.

> Интервью интерактивное — агент задаёт вопросы, ты отвечаешь в Claude Code.
> После завершения интервью агент самостоятельно записывает spec файл.

#### Design — архитектурное решение

Агент `design.md` получает **оба файла** — `research/output.md` и `spec-interview/output.md` — и создаёт `docs/features/{name}/design/output.md`:
- C4 Context + Containers (Mermaid)
- Dataflow (запрос → Fiber → service → repository → pgx → PostgreSQL → ответ)
- Sequence diagram (Mermaid)
- Go слои: handler / service / repository / models + pkg/
- SQL queries (go:embed) + миграции PostgreSQL 18
- Vue: структура feature, Pinia store, API composable
- Integration test design (dockertest, `postgres:18-alpine`)
- ADR с обоснованиями и рисками
- Test plan (unit + integration)

**Пауза:** После создания design/output.md — `AskUserQuestion`: "Дизайн готов. Проверь `docs/features/{name}/design/output.md`. Продолжаем?"

#### Plan — planner/output.md

Агент `planner.md` создаёт `docs/features/{name}/planner/output.md`:
- Строгий порядок до 14 фаз:
  1. Infrastructure (Docker, env vars, CI/CD) — если нужны изменения
  2. Миграции БД (`migrations/`)
  3. SQL queries в `sqls/` (go:embed)
  4. Error handling (`pkg/errorspkg/`) — sentinel errors + supportMessage коды
  5. Models/DTO (`models/` + `models/dto/`)
  6. Validators (`validators/`) + unit тесты
  7. Service + unit тесты
  8. Repository + integration тест (`postgres:18-alpine`)
  9. Handler + Router + integration тест (`handler_integration_test.go`, coverage ≥90%)
  10. DI Registration (`internal/routers/routers.go`)
  11. Swagger Regeneration — `make swagger` + коммит `docs: regenerate swagger`
  12. Vue: `api/` + `types/` (если есть frontend)
  13. Vue: `store/` + `composables/` (если есть frontend)
  14. Vue: `components/` (если есть frontend)
- Каждая фаза = атомарный коммит
- Тесты в той же фазе, что и код
- Каждая фаза не ломает `go build ./...`
- Integration тест всегда в фазе repository, образ `postgres:18-alpine`

**Пауза:** После создания плана — `AskUserQuestion`: "План готов. Проверь `docs/features/{name}/planner/output.md`. Продолжаем?"

#### Executing

1. Запустить Backend (`backend-dev.md`) и Frontend (`frontend-dev.md`) **параллельно**
2. Backend реализует Go фазы из planner/output.md
3. Frontend реализует Vue фазы из planner/output.md
4. После завершения — автоматически quality gates:
   ```bash
   make build
   make lint
   make test
   make test-integration
   cd frontend && npm run type-check && npm run lint && npm run test -- --run
   ```
5. Если gates не проходят — откат в Executing с описанием проблем
6. **Swagger Regeneration (STRICT — оркестратор):**
   - Оркестратор запускает `make swagger`
   - Если exit code != 0 → **блокер**: откат в Executing, backend-dev агент исправляет аннотации (ошибка swag передаётся в контексте)
   - Если OK → отдельный коммит `docs: regenerate swagger` (файлы `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`)
   - **НЕ переходить в Review, пока swagger не обновлён**
7. **Верификация статусов фаз (STRICT — ОБЯЗАТЕЛЬНО перед переходом в Review):**
   - Прочитать `docs/features/{name}/code-plan.md` через Read tool
   - Убедиться, что ВСЕ реализованные фазы имеют статус `completed` в таблице
   - Если какая-либо фаза НЕ `completed` — прочитать файл этой фазы и обновить:
     - `**Status:** \`in_progress\`` или `**Status:** \`pending\`` → `**Status:** \`completed\``
     - Все `- [ ]` в Definition of Done → `- [x]`
     - Статус в таблице `code-plan.md` → `completed`
   - **НЕ переходить в Review, пока все статусы не обновлены**

#### Review — code review

Агент `reviewer.md` получает:
- Все реализованные файлы фичи
- `docs/features/{name}/design/output.md`
- `docs/features/{name}/research/output.md` (секция "Паттерны в коде")

Проверяет по 15-секционному чеклисту: архитектура, Fiber v3 корректность, validators, error handling, pgx/v5, auth&security, integration tests, миграции, внешние интеграции, инфраструктура, Vue/TS, coverage, code quality, паттерны, **swagger coverage**.

Сохраняет результат в `docs/features/{name}/reviewer/output.md`.

- Если есть **Блокеры** → откат в Executing с описанием из `docs/features/{name}/reviewer/output.md`
- Только после **APPROVED** → переход в Validation

#### Report — содержимое

- Название фичи и дата
- Краткое описание задачи
- Итоги Research (сводка консилиума)
- Дизайн-решения (ADR)
- Plan (фазы)
- Что реализовано (файлы, модули)
- Результаты Validation (тесты, платформы)
- Проблемы и откаты (если были)
- Статус: Done / Частично

---

## 7. Профиль: Поиск бага

### Tier-specific artifacts (см. 4.5)

Tier для бага определяется из Triage и учитывает риск (`risk`), число затронутых файлов, `rootCauseType`.

| Стадия | S | M | L |
|---|---|---|---|
| Reproduce | inline в `bug-researcher/output.md` (1–2 шага) | `reproduce/output.md` | полный `reproduce/output.md` |
| Diagnose (`bug-researcher`) | как BUG-001 (~110 строк), короткий отчёт | стандарт | полный + **Monitoring Spec для recurrence detection** (если фикс трогает hot path или добавляет новые spans) |
| Fix | без изменений, роутинг по `rootCauseType` | без изменений | без изменений |
| Review | Slim (5 секций) | Standard (10) | Full (15, infra-reviewer при затрагивании инфры) + Sec 16 Operability **только если L-tier баг добавил Monitoring Spec** |
| Deploy | как сейчас, условие — `rootCauseType ∈ {Config-on-Host, Infra-Template-Missing-Var, Deploy-Drift}` или Fix трогал `infrastructure/**`/`.github/workflows/**` | — | — |
| Report | 1 абзац | стандарт | полный |

Hard invariants сохраняются на всех tier.

**PRD НЕ создаётся для bug-профиля.** Баг — это починка регрессии или некорректного поведения, бизнес-цель уже была зафиксирована при создании оригинальной фичи. Если фикс меняет UX/поведение по требованию пользователя (например, "теперь показывать предупреждение перед удалением") — это не баг, переключиться на Бизнес-фичу профиль и начать с PRD.

### Инварианты стека (нельзя нарушать)

| Слой | Технологии | Fix агент |
|---|---|---|
| Backend | Go 1.26, Fiber v3, PostgreSQL 18, pgx/v5, go:embed | `backend-dev.md` |
| Frontend | Vue 3, TypeScript, Pinia | `frontend-dev.md` |
| Ansible / Docker / CI | hosts.yml, group_vars, host_vars, `.env.j2`, docker-compose | `infra-provisioning.md`, `infra-deploy.md` |
| Docs | CLAUDE.md, README, `docs/**` | general-purpose |

### Bug Hunting Workflow (STRICT)

#### Стадии
1. **Reproduce** — воспроизведение бага
2. **Diagnose** — диагностика корневой причины (`bug-researcher.md` + `rootCauseType`)
3. **Fix** — исправление (роутинг по `rootCauseType`)
4. **Review** — code review (dev-reviewer и/или infra-reviewer по домену)
5. **Validation** — проверка фикса + регрессии + инфра-проверки
6. **Deploy** — условная стадия (если правки затронули инфру или `rootCauseType` инфровый)
7. **Report** — отчёт
8. **Done** — завершено

#### Разрешённые переходы
```
Reproduce  → Diagnose
Reproduce  → Report         (не воспроизводится — отчёт с пометкой)
Diagnose   → Fix
Diagnose   → Reproduce      (нужно воспроизвести иначе)
Diagnose   → Report         (только диагностика, фикс не нужен)
Fix        → Review
Fix        → Diagnose       (фикс выявил другую причину)
Review     → Validation     (APPROVED)
Review     → Fix            (блокеры в reviewer/output.md)
Validation → Deploy         (rootCauseType инфровый ИЛИ Fix трогал infrastructure/**/workflows)
Validation → Report         (чистый code-only фикс — Deploy пропускается)
Validation → Fix            (фикс не работает)
Validation → Diagnose       (причина была другой)
Deploy     → Report         (success)
Deploy     → Fix            (failure; если Deploy→Fix→Deploy зациклился ≥2 раз — Report partial)
Deploy     → Validation     (после фикса нужен re-run тестов)
Report     → Done
```
Все остальные переходы ЗАПРЕЩЕНЫ.

#### Субагенты по стадиям

| Стадия | Агент | Модель | Запускается при | Что делает |
|---|---|---|---|---|
| Reproduce | Bash + Chrome MCP | sonnet | всегда | Воспроизводит баг |
| Diagnose | `bug-researcher.md` | opus | всегда | Определение слоя, цепочка конфига, `rootCauseType` |
| Fix (Code) | `backend-dev.md` / `frontend-dev.md` | opus | `rootCauseType ∈ {Code, Test-Gap}` | Go / Vue фикс |
| Fix (Docs) | general-purpose | haiku | `rootCauseType = Doc-Gap` | Обновление CLAUDE.md / README / `docs/**` |
| Fix (Config) | `infra-provisioning.md` | opus | `rootCauseType = Config-on-Host` | group_vars / host_vars / vault + ansible apply |
| Fix (Template) | `infra-provisioning.md` (+ `infra-deploy.md`) | opus | `rootCauseType = Infra-Template-Missing-Var` | `.env.j2` / vars / compose |
| Fix (Deploy) | `infra-deploy.md` (+ `infra-provisioning.md`) | opus | `rootCauseType = Deploy-Drift` | workflows / compose / upstream |
| Review (dev) | `reviewer.md` | opus | Fix трогал `internal/**` / `config/**` / `frontend/**` | 15-секционный чеклист |
| Review (infra) | `infra-reviewer.md` | opus | Fix трогал `infrastructure/**` / `.github/workflows/**` | 10-секционный чеклист (rollback plan, secrets, OWASP container, CI/CD integrity) |
| Validation | Bash + Chrome MCP | sonnet | всегда | Dev gates (build/lint/test/integration) + Infra gates (`ansible --syntax-check`, `docker compose config`, `promtool`) |
| Deploy | skill `/deploy` | — | инфра-rootCauseType или Fix трогал infrastructure/CI | Автотэг `{env}-NNN`, push, `gh run watch`, smoke-check |
| Report | general-purpose | haiku | всегда | Отчёт |

> **Mixed-фикс** (Code + Infra в одном баге): Fix идёт последовательно Code → mini-gates (`go build`, `go vet`, `npm run type-check`) → Infra. Review — infra-reviewer первый, затем dev-reviewer. Оба должны дать APPROVED.
>
> **Fallback:** если `bug-researcher/output.md` не содержит `rootCauseType` (старый формат) — роутер Fix падает на поле `Слой` (Backend / Frontend / Infra / Несколько) и ведёт себя как раньше.

#### Reproduce — воспроизведение

1. Получить описание бага
2. `AskUserQuestion` — платформа (если не указана): Backend / Web
3. Воспроизвести через Chrome MCP (Web) или Bash (Backend)
4. Сохранить в файл:

```
docs/features/{name}/reproduce/output.md
```

```markdown
# Reproduce: <описание бага>
Платформа: Backend / Web
Статус: Воспроизведён / Не воспроизведён

## Входные данные
- Описание: ...
- Stacktrace/лог: ...
- HTTP запрос/ответ: ...

## Шаги воспроизведения
1. ...
2. ...
3. → Ожидаемый результат: X, Фактический результат: Y

## Go специфика (если backend)
- Goroutine dump (если deadlock)
- pgx/v5 error code
- Fiber error response

## Vue специфика (если web)
- Console errors
- Network tab (статус, тело ответа)
- Pinia state в момент ошибки
```

5. Если НЕ воспроизводится после 3 попыток — `AskUserQuestion` для уточнения условий или Report "Не воспроизведён"

**ВАЖНО:** Перед каждой попыткой — ПЕРЕЧИТАТЬ `docs/features/{name}/reproduce/output.md` через Read tool. Не повторять уже проверенные сценарии.

#### Diagnose — Bug Researcher Agent

Используется специализированный агент `bug-researcher.md`:

1. **Шаг 0:** Запуск существующих тестов (`make test`, `make test-integration`)
2. **Определение слоя** по симптомам (pgx error → Backend, Vue warning → Frontend, 502 → Infra)
3. **Параллельный запуск** только релевантных sub-agents:
   - Backend Bug Investigator — стектрейс, error flow, git blame, SQL, безопасность
   - Frontend Bug Investigator — console errors, Pinia state, reactivity
   - Infra Bug Investigator — Docker, PostgreSQL connections, CI, env vars

**Результат сохраняется в:** `docs/features/{name}/bug-researcher/output.md`
- Root cause гипотеза с доказательствами
- Затронутые файлы с номерами строк
- Рекомендация для Fix
- Проверка фикса + regression тесты

#### Fix — роутинг по `rootCauseType`

Оркестратор читает `docs/features/{name}/bug-researcher/output.md` и парсит поле `**Тип причины (rootCauseType):**`. Матрица маршрутизации — см. таблицу «Субагенты по стадиям» выше.

**Quality gates** по домену:
- Code-фикс (backend-dev / frontend-dev): `make build`, `make lint`, `make test`, `make test-integration`, `cd frontend && npm run type-check && npm run lint && npm run test -- --run`.
- Infra-фикс (infra-provisioning / infra-deploy): `ansible-playbook ... --syntax-check`, `docker compose config --quiet`, `promtool check rules` (если затронуты alert rules).

**Swagger Regeneration (STRICT — оркестратор):** применяется только если фикс правил Go-код с swag-аннотациями.
- Оркестратор запускает `make swagger`.
- Если exit code != 0 → **блокер**: откат в Fix, backend-dev исправляет аннотации.
- Если OK → отдельный коммит `docs: regenerate swagger`.
- **НЕ переходить в Review, пока swagger не обновлён.**

**Mixed-флоу (Code → Infra):**
1. Code-фаза → mini-gates (`go build ./...`, `go vet ./internal/...`, `npm run type-check` если applicable).
2. Если mini-gates упали → откат в Fix, Infra-фаза НЕ стартует.
3. Если OK → Infra-фаза.

#### Deploy — условная стадия

**Предикат запуска:** `rootCauseType ∈ {Config-on-Host, Infra-Template-Missing-Var, Deploy-Drift}` ИЛИ Fix-коммиты затронули `infrastructure/**` / `.github/workflows/**`. Иначе — пропуск сразу в Report.

**Environment:**
- По умолчанию — `dev`. Deploy-стадия вызывает skill `/deploy dev` (skill сам автогенерирует следующий `dev-NNN`, пушит, слушает `deploy.yml` workflow через `gh run watch`).
- `prod` — ТОЛЬКО при явном указании пользователя в исходном запросе. Перед `/deploy prod` — пауза, `AskUserQuestion` со списком изменённых файлов и предлагаемым тегом. Без подтверждения — НЕ деплоить.

**Smoke-check** после успешного blue-green swap: если в `docs/features/{name}/reproduce/output.md` есть curl-команда / URL, повторить запрос и зафиксировать код ответа.

**Сохранить** `docs/features/{name}/deploy/output.md` (тег, slot before→after, image, workflow URL, smoke result, статус).

**При failure** — откат в Fix. Если Deploy→Fix→Deploy зациклился ≥2 раз — стоп, `Report: partial`, эскалация пользователю.

#### Report — содержимое

- Название бага и дата
- Описание проблемы
- Шаги воспроизведения
- Root cause + `rootCauseType`
- Что исправлено (файлы, строки, описание фикса — разбивка по доменам dev/infra)
- Результаты Review (dev-reviewer и/или infra-reviewer)
- Результаты Validation (тесты, платформы, регрессии)
- **Deploy summary** (если Deploy запускался): tag, slot, image, workflow URL, smoke check
- Проблемы и откаты (если были)
- Статус: Fixed / Not Reproducible / Partially Fixed / Won't Fix

---

## 7.5. Профиль: Миграция/Рефакторинг

### Tier-specific artifacts (см. 4.5)

| Стадия | S | M | L |
|---|---|---|---|
| Research | 1 sub-agent | 3 | 5 |
| **Spec Interview (PRD-first)** | inline, ≤5 Q (без PRD) | файл, 5–10 Q + **PRD только если миграция user-facing** | полный + **PRD обязателен если есть user-facing impact** (например, hard cutover endpoint) |
| Migration Design | фазы + rollback, без C4/swimlane | + Mermaid «до/после», Blast Radius, **план мониторинга миграции (метрики прогресса backfill, алерт на остановку >5 мин)** | + swimlane, ADR для каждой развилки, **Performance Budget на backfill (target throughput rows/sec) + Rollout Strategy (если dual-prefix/feature-flag период)** |
| Plan | inline | `planner/output.md` | `planner/output.md` + milestones |
| Review | Slim | Standard + Sec "PRD linkage" если PRD создан | Full + Sec 16 Operability + Sec 17 PRD ↔ ADR (если PRD создан) |
| Validation | smoke + unit + build | + integration + **наблюдение метрик прогресса миграции в Jaeger/Grafana** | + load/migration dry-run + **post-migration p95 latency vs Performance Budget** |
| Report | 1 абзац | стандарт | полный + **сравнение фактических метрик миграции с Performance Budget targets** |

**Rollback обязателен для каждой фазы независимо от tier** (миграции всегда `risk ∈ {irreversible, data-migration}`).

**PRD для миграции — особый случай.** Большинство рефакторингов и миграций не имеют user-facing impact (внутренний рефакторинг, переход с pgx v4→v5, изменение паттерна). Для них PRD не нужен — достаточно технической migration-design. PRD обязателен только если миграция меняет поведение, видимое пользователю: hard cutover URL, deprecation endpoint, изменение формата ответа, удаление функциональности. В таком случае PRD объясняет, **зачем** ломать совместимость и как клиент адаптируется.

### Инварианты стека (нельзя нарушать)

Целевой стек фиксирован. Предлагать альтернативы ЗАПРЕЩЕНО.

| Целевой слой | Технологии |
|---|---|
| Backend | Go 1.26, Fiber v3, PostgreSQL 18, pgx/v5, go:embed, dockertest/v3 |
| Frontend | Vue 3, TypeScript, Pinia, Vitest |

### Migration Workflow (STRICT)

#### Стадии
1. **Research** — исследование текущего кода
2. **Spec Interview** — фиксация стратегии миграции, ограничений, рисков
3. **Migration Design** — архитектура перехода (текущее → целевое)
4. **Plan** — planner/output.md с фазами (каждая = работоспособное состояние)
5. **Executing** — реализация по плану
6. **Review** — code review реализации
7. **Validation** — тесты, сборка, E2E
8. **Report** — отчёт
9. **Done**

#### Разрешённые переходы
```
Research          → Spec Interview
Spec Interview    → Migration Design
Spec Interview    → Research           (нужен доп. контекст)
Migration Design  → Plan
Migration Design  → Spec Interview     (найдены новые неопределённости)
Plan              → Executing
Executing         → Review
Review            → Validation
Review            → Executing          (блокеры в reviewer/output.md)
Validation        → Report
Validation        → Executing          (дефект)
Validation        → Migration Design   (архитектурная проблема)
Report            → Done
```
Все остальные переходы ЗАПРЕЩЕНЫ.

#### Субагенты по стадиям

| Стадия | Агент | Модель | Что делает |
|---|---|---|---|
| Research | Консилиум (research.md) | opus | Анализ текущего и целевого состояния |
| Spec Interview | `spec-interview.md` | opus | Стратегия, ограничения, rollback план |
| Migration Design | `migration-design.md` | opus | Текущее→целевое, фазы, rollback, ADR |
| Plan | `planner.md` | opus | planner/output.md (каждая фаза = компилируемое состояние) |
| Executing Backend | `backend-dev.md` | opus | Go фазы |
| Executing Frontend | `frontend-dev.md` | opus | Vue фазы (если применимо) |
| Review | `reviewer.md` | opus | 15-секционный code review |
| Mobile API Review | `mobile-api-reviewer.md` | opus | API/Swagger глазами клиента (parallel с design/code review). Trigger: только если миграция трогает HTTP-поверхность |
| Validation | Bash + Chrome MCP | sonnet | Сборка, тесты, E2E (+ mobile-api-reviewer на validation-стадии если есть live dev URL) |
| Report | general-purpose | haiku | Отчёт |

> Executing Backend и Frontend запускаются **параллельно** через Task tool.

#### Executing — Swagger + верификация статусов (STRICT)

Аналогично профилю "Бизнес-фича":
1. После quality gates оркестратор запускает `make swagger`
   - Если exit code != 0 → **блокер**: откат в Executing, backend-dev исправляет аннотации
   - Если OK → отдельный коммит `docs: regenerate swagger`
2. Оркестратор ОБЯЗАН проверить, что все реализованные фазы в `code-plan.md` имеют статус `completed`, Definition of Done отмечены. Если нет — обновить через Edit tool перед переходом в Review.

#### Критические правила Migration Design
- Каждая фаза оставляет проект **работоспособным** (`make build` проходит после каждой фазы)
- Rollback plan обязателен для каждой фазы
- Открытые вопросы из spec → ответить через ADR (не игнорировать)
- Backward compatibility — описать даже если не нужна (указать почему)

#### Review — code review

Аналогично профилю "Бизнес-фича": агент `reviewer.md` по 15-секционному чеклисту.
Сохраняет в `docs/features/{name}/reviewer/output.md`.
Блокеры → откат в Executing. APPROVED → переход в Validation.

#### Report — содержимое

- Что было → что стало (diff компонентов)
- Фазы миграции и их статусы
- Rollback ситуации (если были)
- Backward compatibility изменения
- Результаты Validation (тесты, платформы, регрессии)
- Статус: Done / Partial / Rolled Back

---

## 7.6. Профиль: Инфра-изменение

### Tier-specific artifacts (см. 4.5)

Инфра-профиль уже использовал S/M/L. Критерии гармонизированы с разделом 4.5 — те же признаки, то же правило `max`. Матрица артефактов:

| Секция / артефакт | S | M | L |
|---|---|---|---|
| Обязательные секции design/output.md | 1, 8, 11 | 1, 2, 7, 8, 9, 11 | все 11 секций |
| Mermaid «до/после» | — | да | да |
| Network Topology | — | — | да |
| Blast Radius таблица | — | да | да |
| Swimlane диаграмма | — | — | да (или при ≥5 фаз/ролей) |
| WAL-G pre-check | — | — | да (при restart БД) |
| Rollback Plan (Sec 8) | **всегда** | **всегда** | **всегда** |
| Review checklist | Slim (секции 1, 2, 9) | Standard (1, 2, 3, 4, 8, 9) | Full (все 9) |

Syntax-проверки (`ansible --syntax-check`, `docker compose config`, `promtool`) — всегда, независимо от tier.

### Инварианты стека (нельзя нарушать)

| Слой | Технологии | Агент |
|---|---|---|
| CI/CD | GitHub Actions, Docker, Dockerfile | `infra-deploy.md` |
| Provisioning | Ansible playbooks, roles, inventory | `infra-provisioning.md` |
| Monitoring | Prometheus, Grafana, Loki, Alertmanager, Nginx | `infra-monitoring.md` |
| Database HA | Patroni, etcd, WAL-G, PostgreSQL | `infra-db.md` |

### Ключевые отличия от других профилей

- **Полное разделение** — dev-агенты (backend-dev, frontend-dev, research, planner, reviewer) НЕ используются
- **Отдельный оркестратор** `/infra` — не трогать `/pipeline`
- **Docs** — `docs/infra/{name}/` (не `docs/features/{name}/`)
- **Визуализация** — `infra-design.md` обязательно генерирует Mermaid диаграммы

### Infrastructure Workflow (STRICT)

#### Стадии

```
Research → Design → Plan → Executing → Review → Validation → Report → Done
```

#### Разрешённые переходы

```
Research    → Design
Design      → Plan
Plan        → Executing
Executing   → Review
Review      → Validation    (только APPROVED)
Review      → Executing     (CHANGES REQUESTED — с блокерами)
Validation  → Report
Validation  → Executing     (дефект найден)
Report      → Done
```

Все остальные переходы ЗАПРЕЩЕНЫ.

#### Субагенты по стадиям

| Стадия | Агент | Модель | Что делает |
|---|---|---|---|
| Research | `infra-researcher.md` | opus | SSH/config анализ, drift detection, CI/CD история |
| Design | `infra-design.md` | opus | Mermaid диаграммы, rollback plan, blast radius |
| Plan | `infra-planner.md` | opus | Фазы (каждая = стабильная инфра + rollback) |
| Executing (CI/CD, Docker) | `infra-deploy.md` | opus | `.github/workflows/`, `docker-compose*.yml`, `Dockerfile*` |
| Executing (Ansible) | `infra-provisioning.md` | opus | `infrastructure/ansible/` |
| Executing (Monitoring/Nginx) | `infra-monitoring.md` | opus | Prometheus, Grafana, Loki, Nginx |
| Executing (Database) | `infra-db.md` | opus | Patroni, WAL-G, replication |
| Review | `infra-reviewer.md` | opus | 8-секционный инфра-чеклист |
| Validation | Bash | sonnet | `docker compose config`, `ansible --syntax-check`, `promtool check` |
| Report | general-purpose | haiku | Отчёт |

> Executing агенты запускаются **параллельно** если задача затрагивает несколько зон (например, infra-deploy + infra-monitoring).

#### Инфра-специфичные правила

- **Rollback plan** обязателен для каждой фазы в planner/output.md
- **Patroni** — `synchronous_mode: true` в prod не трогать без явного запроса
- **Vault** — реальные значения секретов НИКОГДА не писать в файлы
- **Каждая фаза** = работоспособное состояние инфры после применения
- **Порядок фаз**: БД → App → Monitoring → Nginx (dependency order)

#### Slash-команды (быстрые операции)

| Команда | Назначение |
|---|---|
| `/infra <задача>` | Полный инфра-пайплайн |
| `/infra-diagram [topic]` | Генерация Mermaid диаграмм (network, dataflow, cicd, secrets, monitoring, bluegreen, patroni, all) |
| `/deploy <env> [tag]` | Быстрый деплой с подтверждением |
| `/rollback <env>` | Откат к предыдущей версии |
| `/infra-check [env]` | Health-check всех сервисов |

#### Validation — инфра-проверки

```bash
# Docker Compose syntax
docker compose -f infrastructure/{env}/docker-compose.yml config --quiet

# Ansible syntax check
ansible-playbook infrastructure/ansible/playbooks/setup.yml --syntax-check \
  -i infrastructure/ansible/inventory/hosts.yml

# Prometheus rules
promtool check rules infrastructure/{env}/prometheus/rules/alerts.yml

# Nginx (если есть доступ)
nginx -t -c /path/to/nginx.conf
```

#### Review — 9-секционный чеклист `infra-reviewer.md`

1. Secrets & Security — нет реальных секретов в файлах
2. Rollback Plan — полон и конкретен для каждой фазы
3. Syntax & Config Validity — все проверки задокументированы
4. Security Hardening — rate limits, порты, auth
5. Environment Scope — изменения в нужных окружениях
6. Backward Compatibility — существующие сервисы не сломаны
7. Health Checks & Observability — мониторинг покрывает изменения
8. Database Safety — Patroni/WAL-G инварианты соблюдены
9. OWASP / Container & CI/CD Security — Docker, GitHub Actions, Supply Chain

#### Report — содержимое

- Задача и дата
- Исследование (состояние до изменения)
- Дизайн (ссылки на Mermaid диаграммы)
- Фазы реализации (статусы)
- Validation результаты
- Rollback ситуации (если были)
- Статус: Done / Partially Done / Rolled Back

---

## 8. Контекст проекта: HoReKen Events

> Этот раздел читается агентами для понимания домена, схемы БД, API и интеграций.
> Агенты универсальны — специфика живёт здесь.

### Домен

**HoReKen Events** — платформа поиска работы в сфере HoReCa (Hotel, Restaurant, Cafe).

**Типы пользователей:**
- **Кандидат** — специалист (бармен, официант, повар, менеджер), ищет работу
- **Работодатель** — заведение (бар, ресторан, отель), публикует вакансии
- Один аккаунт может быть одновременно кандидатом И работодателем (`role_flags: ["candidate", "employer"]`)
- **Admin** — внутренний пользователь, верифицирует работодателей через `X-Admin-Secret`

### Auth & Безопасность

- **JWT** (`golang-jwt/jwt/v5`): access token (3 дня) + UUID refresh token (30 дней)
  - Payload: `user_id`, `login`, `exp`
  - Header: `Authorization: Bearer <accessToken>`
- **Telegram Login**: HMAC-SHA256 верификация подписи + проверка `auth_date < 86400 сек`
  - Login формат: `tg_<telegram_id>`
- **bcrypt** (`cost 12`) для хэширования паролей (Telegram-пользователи без пароля)
- **Минимальная длина пароля**: 8 символов
- **Rate limiting**: nginx (30 req/s API, 5 req/m admin), app-level rate limiter отсутствует
- **Admin middleware**: заголовок `X-Admin-Secret` (если пустой → 403)

### Push-уведомления (FCM)

- Firebase Cloud Messaging (`firebase.google.com/go/v4`)
- **Multi-device** (фича `fcm-device-tokens-multiplatform`): токены хранятся в отдельной таблице `device_tokens`, до 10 устройств на пользователя (FIFO eviction после превышения)
- Endpoints: `PUT /api/v1/auth/device-token` (upsert), `DELETE /api/v1/auth/device-token` (logout-on-device, см. ADR-007)
- Fan-out: каждое уведомление рассылается на ВСЕ device_tokens получателя; UNREGISTERED/INVALID_ARGUMENT → async cleanup (TTL-дедуп 300с)
- Отправляется при: новый отклик (работодателю), изменение статуса (кандидату), новая вакансия (кандидатам по категории), feature_request status/comment изменения
- Ошибка FCM не блокирует основную операцию — только логируется (ADR-005, best-effort)
- Если у пользователя 0 device_tokens — push не отправляется (не ошибка)

### Переменные окружения

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=user
DB_PASSWORD=password
DB_NAME=horeken
JWT_SECRET=your-secret-key
TELEGRAM_BOT_TOKEN=1234567890:AAFxxx
ADMIN_SECRET=your-admin-secret
FIREBASE_CREDENTIALS_JSON={"type":"service_account",...}
LOG_LEVEL=info
PORT=8080
ENV=development
# OpenTelemetry / Tracing
OTEL_EXPORTER_OTLP_ENDPOINT=   # пусто = noop (local), http://jaeger:4317 = Jaeger gRPC
OTEL_TRACES_SAMPLER_ARG=1.0    # 1.0 = dev/staging, 0.1 = prod
```

### Схема БД (PostgreSQL 18)

```sql
-- users
id            VARCHAR(64) PRIMARY KEY   -- email-based или "tg_<telegram_id>"
login         VARCHAR(50) UNIQUE NOT NULL
password      VARCHAR(255)              -- bcrypt, NULL для Telegram users
first_name    VARCHAR(50)
last_name     VARCHAR(50)
phone         VARCHAR(20)
image_url     VARCHAR(255)
city_id       VARCHAR(64) NULL REFERENCES cities(id) ON DELETE RESTRICT
is_new        INT DEFAULT 1
role_flags    JSONB                     -- ["candidate", "employer"]
telegram_url  VARCHAR(255)
personal_data_consent BOOLEAN NOT NULL DEFAULT FALSE  -- issue #45 ADR-001
terms_accepted        BOOLEAN NOT NULL DEFAULT FALSE  -- issue #45 ADR-001
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- INDEX idx_users_city_id (city_id) — резолв города пользователя в snippet'ах

-- tokens (1:1 → users)
user_id       VARCHAR(64) PRIMARY KEY REFERENCES users(id)
access_token  VARCHAR(300) NOT NULL
refresh_token VARCHAR(300) NOT NULL
expires_at    TIMESTAMPTZ NOT NULL
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- candidates (1:1 → users)
user_id           VARCHAR(64) PRIMARY KEY REFERENCES users(id)
city_id           VARCHAR(64) NULL REFERENCES cities(id) ON DELETE RESTRICT
categories        JSONB      -- ["bartender", "waiter", "dj", ...]
skills            JSONB
hourly_rate       DECIMAL(10,2)
shift_rate        DECIMAL(10,2)
experience_months INT
portfolio_media   JSONB
equipment         JSONB
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- employers (1:1 → users)
user_id       VARCHAR(64) PRIMARY KEY REFERENCES users(id)
city_id       VARCHAR(64) NULL REFERENCES cities(id) ON DELETE RESTRICT
company_name  VARCHAR(255)
company_type  VARCHAR(100)
address       VARCHAR(255)
description   TEXT
verified_bool BOOLEAN DEFAULT FALSE
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- categories (справочник категорий вакансий)
id          VARCHAR(64) PRIMARY KEY
slug        VARCHAR(50) NOT NULL UNIQUE  -- a-z0-9-, lowercase, длина 2–50
name        VARCHAR(100) NOT NULL        -- отображаемое имя на русском
sort_order  INT NOT NULL DEFAULT 0       -- порядок в публичном списке
is_active   BOOLEAN NOT NULL DEFAULT TRUE
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- INDEX idx_categories_active_sort (is_active, sort_order, name) — публичный листинг

-- vacancies
id                         VARCHAR(64) PRIMARY KEY
employer_id                VARCHAR(64) NOT NULL REFERENCES users(id)
title                      VARCHAR(255) NOT NULL
description                TEXT
category_id                VARCHAR(64) NULL REFERENCES categories(id) ON DELETE RESTRICT
city_id                    VARCHAR(64) NULL REFERENCES cities(id) ON DELETE RESTRICT
hourly_rate                DECIMAL(10,2)
shift_rate                 DECIMAL(10,2)
required_skills            JSONB
required_experience_months INT
status                     VARCHAR(10) NOT NULL DEFAULT 'active'  -- 'active'|'inactive'
created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- INDEX idx_vacancies_category_id (category_id) — фильтрация по категории

-- applications
id           VARCHAR(64) PRIMARY KEY
vacancy_id   VARCHAR(64) NOT NULL REFERENCES vacancies(id)
candidate_id VARCHAR(64) NOT NULL REFERENCES users(id)
status       VARCHAR(10) NOT NULL DEFAULT 'new'  -- 'new'|'viewed'|'accepted'|'rejected'
created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- notifications
id         VARCHAR(64) PRIMARY KEY
user_id    VARCHAR(64) NOT NULL REFERENCES users(id)
type       VARCHAR(50) NOT NULL
title      VARCHAR(255) NOT NULL
body       TEXT
is_read    BOOLEAN NOT NULL DEFAULT FALSE
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()

-- device_tokens (фича fcm-device-tokens-multiplatform; миграция 000003)
id            VARCHAR(64) PRIMARY KEY
user_id       VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE
device_token  VARCHAR(512) NOT NULL
platform      VARCHAR(16) NOT NULL CHECK (platform IN ('ios','android','unknown'))
app_version   VARCHAR(32) NULL
locale        VARCHAR(16) NULL
bundle_id     VARCHAR(128) NULL
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- UNIQUE (user_id, device_token) — для ON CONFLICT upsert
-- INDEX idx_device_tokens_user_id (user_id) — fan-out lookup в notification service
```

### API Контракт

```
# Аутентификация
POST   /api/v1/auth/registration       — создаёт user + candidate + employer профили
POST   /api/v1/auth/login
POST   /api/v1/auth/telegram           — Telegram Login (HMAC-SHA256)
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout             [JWT]
PUT    /api/v1/auth/device-token       [JWT] — register/upsert FCM token (multi-device, до 10/user, FIFO eviction)
DELETE /api/v1/auth/device-token       [JWT] — logout-on-device (body: device_token; см. ADR-007)
PUT    /api/v1/auth/password           [JWT]

# Admin
PUT    /api/v1/admin/employers/:id/verify  [X-Admin-Secret]

# Профили
GET    /api/v1/profiles/candidate      [JWT]
PUT    /api/v1/profiles/candidate      [JWT]
GET    /api/v1/profiles/employer       [JWT]
PUT    /api/v1/profiles/employer       [JWT]
GET    /api/v1/profiles/candidates/:id
GET    /api/v1/profiles/employers/:id
GET    /api/v1/profiles/:id            — публичный (PublicProfileResponse, ADR-004/006)

# Категории
POST   /api/v1/admin/categories          [X-Admin-Secret]
PUT    /api/v1/admin/categories/:id      [X-Admin-Secret] — slug immutable
GET    /api/v1/admin/categories          [X-Admin-Secret] — все, включая is_active=false
GET    /api/v1/admin/categories/:id      [X-Admin-Secret]
GET    /api/v1/categories                — публичный, только is_active=true

# Города
POST   /api/v1/admin/cities              [X-Admin-Secret]
PUT    /api/v1/admin/cities/:id          [X-Admin-Secret] — slug immutable
GET    /api/v1/admin/cities              [X-Admin-Secret] — все, включая is_active=false
GET    /api/v1/admin/cities/:id          [X-Admin-Secret]
GET    /api/v1/cities                    — публичный, только is_active=true

# Вакансии
GET    /api/v1/vacancies               — ?category_id=&city_id=&city_slug=&min_rate= (location удалён, city_id приоритетнее city_slug)
GET    /api/v1/vacancies/my            [JWT] — экран «Мои вакансии» работодателя; ?status=active|inactive (опц.), ?limit, ?offset
GET    /api/v1/vacancies/:id
POST   /api/v1/vacancies               [JWT]
PUT    /api/v1/vacancies/:id           [JWT]
DELETE /api/v1/vacancies/:id           [JWT]

# Отклики
POST   /api/v1/vacancies/:id/apply     [JWT]
GET    /api/v1/applications            [JWT] → {as_candidate:[], as_employer:[]}
PUT    /api/v1/applications/:id/status [JWT]

# Уведомления
GET    /api/v1/notifications           [JWT]
PUT    /api/v1/notifications/:id/read  [JWT]
DELETE /api/v1/notifications/:id       [JWT]
```

### Формат ошибок API

```json
{
  "status": 400,
  "title": "Ошибка",
  "message": "Описание для пользователя",
  "supportMessage": "1",
  "timestamp": "2026-04-04T12:00:00Z"
}
```

### Коды ошибок (supportMessage)

| Код | Описание |
|-----|----------|
| `1` | Не удалось создать пользователя |
| `2` | Пользователь не найден |
| `3` | Не удалось сгенерировать токен |
| `4` | Не удалось сохранить токен |
| `5` | Refresh токен не найден |
| `6` | Невалидный refresh токен |
| `7` | Refresh токен истёк |
| `8` | Refresh токен обязателен |
| `9` | Не удалось создать профиль кандидата |
| `10` | Не удалось создать профиль работодателя |
| `11` | Профиль кандидата не найден |
| `12` | Профиль работодателя не найден |
| `13` | Необходима авторизация |
| `14` | Вакансия не найдена |
| `15` | Нет прав на редактирование вакансии |
| `16` | Отклик не найден |
| `17` | Нет прав на изменение статуса отклика |
| `18` | Повторный отклик на вакансию |
| `19` | Уведомление не найдено |
| `20` | Нет прав на изменение уведомления |
| `21` | Неверный текущий пароль |
| `34` | Нельзя откликаться на свою вакансию |
| `35` | Категория не найдена |
| `36` | Категория с таким slug уже существует |
| `37` | Некорректный slug категории (a-z, 0-9, дефис; длина 2–50) |
| `38` | Некорректное название категории |
| `39` | Slug категории нельзя изменить после создания |
| `40` | Навык не найден |
| `41` | Навык с таким slug уже существует |
| `42` | Некорректный slug навыка (a-z, 0-9, дефис; длина 2–50) |
| `43` | Некорректное название навыка |
| `44` | Slug навыка нельзя изменить после создания |
| `45` | Указан неактивный или несуществующий навык |
| `46` | Дубликат навыка в массиве |
| `47` | Превышен лимит навыков (30) |
| `48` | Город не найден |
| `49` | Город с таким slug уже существует |
| `50` | Некорректный slug города (a-z, 0-9, дефис; длина 2–50) |
| `51` | Некорректное название города |
| `52` | Slug города нельзя изменить после создания |
| `53` | Нельзя отменить принятый отклик |
| `54` | Некорректное значение query-параметра is_urgent |
| `62` | Необходимо согласие на обработку персональных данных и принятие правил сервиса |
| `63` | Некорректное значение query-параметра status (допустимо: active, inactive) |
| `64` | Идея не найдена |
| `65` | Нет прав на изменение идеи |
| `66` | Удалить можно только идею в статусе pending |
| `67` | Нельзя голосовать за свою идею |
| `68` | Нельзя голосовать за завершённые идеи |
| `69` | Превышен лимит создания идей (5 в сутки) |
| `70` | Превышен лимит комментариев (20 в час) |
| `71` | Комментарий не найден |
| `72` | Нет прав на комментарий |
| `73` | Недопустимый переход статуса |
| `74` | Некорректный sort (допустимо: top, new) |
| `75` | Некорректный status filter |
| `76` | Некорректный заголовок идеи (5..120 символов) |
| `77` | Некорректное описание идеи (10..2000 символов) |
| `78` | Некорректный текст комментария (1..1000 символов) |
| `79` | Device token не найден (DELETE: токен отсутствует или принадлежит другому пользователю — единый код, чтобы не утекало "чей токен") |
| `80` | Device token принадлежит другому пользователю (зарезервировано; на handler'ах 0 affected → 79) |
| `81` | Некорректное значение platform (допустимо: ios, android, unknown) |
| `82` | Некорректный формат device_token (длина 10..512) |
| `83` | app_version слишком длинный (max 32 символа) |
| `84` | Некорректное значение locale (max 16 символов) |

### OpenTelemetry / Трейсинг

**Реализация:** OTLP gRPC → Jaeger all-in-one (badger storage).

| Компонент | Файл | Поведение |
|-----------|------|-----------|
| Инициализация | `pkg/tracing/tracing.go` | `Init(ctx, "horeken-events")` — если `OTEL_EXPORTER_OTLP_ENDPOINT` пуст, noop-провайдер (span IDs генерируются, экспорт не происходит) |
| HTTP middleware | `internal/middleware/otel.go` | `middleware.Otel()` — первый в цепочке; W3C TraceContext propagation; `c.Locals("trace_id")` для логов |
| DB | `config/db/config.go` | `otelpgx.NewTracer()` — каждый SQL-запрос = child span (автоматически) |
| FCM | `config/firebase/config.go` | Ручной span: `otel.Tracer("horecenevents/fcm").Start(ctx, "fcm.Send")` |
| Logger | `pkg/logger/logger.go` | `logger.TraceID(ctx)` — извлечь trace_id для корреляции slog ↔ Jaeger |

**Env vars:**
- `OTEL_EXPORTER_OTLP_ENDPOINT` — пусто = noop; `http://jaeger:4317` = Jaeger gRPC
- `OTEL_TRACES_SAMPLER_ARG` — коэффициент сэмплирования: `1.0` (dev), `0.1` (prod)

**Передача контекста (СТРОГО ОБЯЗАТЕЛЬНО):**
- Handler: `ctx := c.Context()` → передать в service
- Service: первый аргумент `ctx context.Context` → передать в repository
- Repository: `pool.QueryRow(ctx, sql, ...)` — ctx обязателен для otelpgx
- **НИКОГДА** не использовать `context.Background()` в handler/service/repository — это обрывает трейс
  (также запрещено линтером `noctx`)

**Ручной span для внешних вызовов (HTTP-клиенты, gRPC, другие интеграции):**
```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/codes"
)

tracer := otel.Tracer("horecenevents/{component}")
ctx, span := tracer.Start(ctx, "operation.Name")
defer span.End()
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

**Логирование с trace_id:**
```go
import "horecenevents/pkg/logger"

slog.InfoContext(ctx, "создан отклик", "trace_id", logger.TraceID(ctx), "id", id)
```

**Соглашение об именовании tracer:**
- `"horecenevents/http"` — HTTP layer (`internal/middleware/otel.go`)
- `"horecenevents/fcm"` — FCM push (`config/firebase/config.go`)
- `"horecenevents/{component}"` — новые компоненты (следовать паттерну)

**DB-запросы инструментируются автоматически через otelpgx — ручные span-ы для pgx не нужны.**

---

## 8.1. OpenAPI Enum Coverage (STRICT)

Каждое поле API с ограниченным набором значений (DB CHECK constraint, business enum, fixed list) ОБЯЗАНО документироваться в OpenAPI как enum.

### Три инварианта

1. **Coverage.** В DTO-структуре поле помечается тегом `enums:"a,b,c"`. Для `@Param` query/path/header — `Enums(a, b, c)` в swag-комментарии. Generic `{object}` / `{array} object` в `@Success` ЗАПРЕЩЕНЫ для DTO с enum-полями — иначе теги теряются в `swagger.json`.

2. **Single source of truth.** Список значений живёт в Go-коде:
   - `internal/features/{name}/constants/constants.go` — feature-specific enum'ы (имя пакета — `constants`).
   - `pkg/errorspkg/codes.go` — `SupportMessageCodes` (общий список).
   Валидаторы (`internal/features/{name}/validators/`) читают **только** из constants. Хардкод в struct-тегах допустим (swag не умеет читать Go-константы из тегов), но синхронизируется тестом.

3. **Sync-тест.** Каждый enum имеет reflection-based тест в `internal/features/{name}/models/*_swag_test.go` (или `pkg/errorspkg/codes_test.go`):
   - извлекает значения из `enums:"..."` тега через `reflect`,
   - сверяет с константой пакета,
   - падает при расхождении с понятным сообщением.

### ErrorResponse дополнительно

`pkg/errorspkg/error_response.go.SupportMessage` имеет полный список кодов (1..39 на момент 2026-04-27) в `enums:` теге **И** swag-аннотации `// @Description` со списком "код — описание". При добавлении нового кода:
1. `pkg/errorspkg/errors.go` — sentinel error + константа `ErrStatus*`.
2. `pkg/errorspkg/codes.go` — добавить код в `SupportMessageCodes`.
3. `pkg/errorspkg/error_response.go` — расширить `enums:` тег и `// @Description` блок.
4. `CLAUDE.md` §8 — обновить таблицу кодов.
5. `make swagger` — регенерировать.

### Limitation swag v1.16.6

Swag не разрешает aliased imports в кросс-пакетных типах: `[]*sharedModels.Application` внутри `applicationService.ApplicationsResponse` приводит к ошибке `cannot find type definition: sharedModels.Application`. Workaround — использовать прямой импорт `models` (без алиаса) или объявить локальный wrapper-тип в handler-пакете.

### Где это закреплено в работе агентов

- **Research** (`.claude/agents/research.md` → Sub-agent 3 Contract Researcher) — аудит существующего покрытия в фазе сбора фактов.
- **Spec Interview** (`.claude/agents/spec-interview.md` → Блок 3) — фиксирует enum-поля фичи.
- **Design** (`.claude/agents/design.md`) — обязательная секция `## Enums и константы` в `design/output.md` (M/L tier; inline в S).
- **Migration Design** (`.claude/agents/migration-design.md`) — наследует §8.1, если миграция трогает CHECK или вводит новое значение.
- **Planner** (`.claude/agents/planner.md`) — Phase 4.5 (Enum Constants) и gate в Phase 11 (Swagger).
- **Backend Dev** (`.claude/agents/backend-dev.md`) — раздел "DTO с enum-полями" с чеклистом и snippet'ами.
- **Reviewer** (`.claude/agents/reviewer.md`) — секция 15 (Swagger Coverage) с тремя блокерами; добавлено в Hard invariants.

Образец полного покрытия: коммит `dd4d53b` (`feat(swagger): add enum values to status, role_flags, supportMessage`).
