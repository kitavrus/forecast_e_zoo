# Phase 07: Validators (формат) + ValidatorEngine + validation_rules.yaml

**Цель:** ввести два уровня валидации:
1. **Формальная** валидация HTTP-запросов через `go-playground/validator/v10` + handler-side декодеры (cursor, ETag, query-параметры). Это Go-код в `validators/`.
2. **Содержательная** severity-валидация бизнес-данных (отрицательные остатки, дубликаты, future event_time) через ValidatorEngine, конфигурируемый YAML-файлом `configs/validation_rules.yaml`. Стартовый набор — 7 правил из ADR-006.

**Commit:** `feat(data_export/validation): формальные валидаторы + severity-движок + validation_rules.yaml`

---

## Files to CREATE

### Формальные валидаторы (для handler-ов)

- `internal/features/data_export/handler/validators/validators.go`:
  - `func NewValidator() *validator.Validate` — настройка validator/v10 + кастомные правила (`uuid`, `iso_date`, `cursor_b64`).
  - `func ParseCursor(s string) (models.Cursor, error)` — декод/проверка → `errorspkg.ErrInvalidCursor` при ошибке.
  - `func ParseEventDateRange(from, to string) (time.Time, time.Time, error)` — `errorspkg.ErrInvalidQuery` если from > to или out-of-range.
  - `func ParseLimit(s string, def, max int) (int, error)`.
- `internal/features/data_export/handler/validators/validators_test.go`:
  - `TestParseCursor_Valid`
  - `TestParseCursor_Invalid_ReturnsErrInvalidCursor`
  - `TestParseEventDateRange_FromGreaterThanTo_ReturnsErrInvalidQuery`
  - `TestParseLimit_Negative_ReturnsErrBadRequest`
  - `TestNewValidator_RejectsBadFormat` (PostExportRequest с `format: "xml"`).

### Severity-движок (`internal/features/data_export/validation/`)

- `validation/engine.go`:
  - `type Rule struct{ ID, Entity, Field, Op, Value string; Severity string /* critical|soft */ }`.
  - `type Engine struct{ rules []Rule, builtin map[string]CheckFunc }`.
  - `type CheckFunc func(payload map[string]any) (ok bool, msg string)`.
  - `func Load(yamlPath string) (*Engine, error)` — парсит YAML.
  - `func (e *Engine) Check(entity string, payload map[string]any) []Violation` — возвращает список нарушений.
  - `type Violation struct{ RuleID, Severity, Message string }`.
- `validation/builtin.go` — встроенные `CheckFunc`:
  - `negative_qty` (qty < 0)
  - `future_event_time` (`event_time > now+15min`)
  - `negative_stock` (qty_on_hand < 0)
  - `duplicate_pk` (внутренний — учёт уже виденных PK в рамках одного load-а; держим состояние в Engine).
  - `missing_required` (универсальный по списку полей)
  - `numeric_range` (универсальный)
  - `regex_match` (для штрихкодов/EAN13)
- `validation/engine_test.go` — unit:
  - `TestEngine_LoadYAML`
  - `TestEngine_NegativeQty_Critical`
  - `TestEngine_FutureEventTime_Critical`
  - `TestEngine_DuplicatePK_Critical`
  - `TestEngine_MissingField_Soft`
  - `TestEngine_NumericRange_Soft`
  - `TestEngine_NoViolations`

### YAML-конфиг

- `configs/validation_rules.yaml` — 7 стартовых правил (по ADR-006):
  ```yaml
  version: 1
  rules:
    - id: receipt_line.qty_negative
      entity: receipt_line
      check: negative_qty
      field: qty
      severity: critical
    - id: receipt_line.event_time_future
      entity: receipt_line
      check: future_event_time
      field: event_time
      severity: critical
    - id: location_stock.qty_negative
      entity: location_stock_snapshot
      check: negative_stock
      field: qty_on_hand
      severity: critical
    - id: products.duplicate_pk
      entity: products
      check: duplicate_pk
      field: id
      severity: critical
    - id: products.barcode_format
      entity: product_barcodes
      check: regex_match
      field: barcode
      pattern: "^[0-9]{8,14}$"
      severity: soft
    - id: location.required_fields
      entity: location
      check: missing_required
      fields: [id, name, type]
      severity: critical
    - id: supply_spec.lead_time_range
      entity: supply_spec
      check: numeric_range
      field: lead_time_days
      min: 0
      max: 90
      severity: soft
  entity_optional:
    - supplier_stock_snapshot
  ```
- `configs/validation_rules_test.go` — `TestYAML_Parses` (загружается без ошибок), `TestYAML_AllRulesHaveSeverity`.

## Files to MODIFY

- `pkg/errorspkg/errors.go` — добавить sentinel: `ErrInvalidCursor` (400, code `bad_request`), `ErrInvalidQuery` (400, code `bad_request`), `ErrInvalidExportFormat` (400, code `bad_request`).
- `pkg/errorspkg/errors_test.go` — кейсы для новых sentinel.
- `go.mod` / `go.sum` — `gopkg.in/yaml.v3`.

## SQL/Migrations

— нет.

## Run after

```bash
go mod tidy
make build
make test-unit
make lint
```

## Tests in this phase

- 5 тестов в `validators_test.go`
- 7 тестов в `engine_test.go`
- 2 теста в `validation_rules_test.go`
- 3 новых sentinel-теста в `errorspkg`

## Definition of Done

- [ ] Формальные валидаторы покрывают cursor, event_date range, limit, формат запросов.
- [ ] ValidatorEngine загружает `validation_rules.yaml` без ошибок.
- [ ] 7 built-in checks работают и покрыты тестами.
- [ ] `entity_optional: [supplier_stock_snapshot]` обрабатывается (engine не падает на отсутствии правил).
- [ ] Все sentinel-ошибки (`ErrInvalidCursor`, `ErrInvalidQuery`, `ErrInvalidExportFormat`) определены и покрыты тестами.
- [ ] `make build` / `make test-unit` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/validation): ...`.
