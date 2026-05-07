# Phase 10: Loader service

**Цель:** реализовать оркестрацию суточного load-а (без cron-обвязки — она в фазе 12). Сервис принимает `SourceReader` + `Repository` + `ValidatorEngine`, выполняет цикл: master → facts. Для каждой сущности: `Iter.Next` → mapper → `Engine.Check` → critical → reject_log; иначе UPSERT в staging. После всех — flip snapshot. На ошибке — mark failed. Quality threshold: `lines_failed/lines_total > 1%` → fail без flip.

**Commit:** `feat(data_export/loader): orchestration service + unit tests (mock SourceReader, mock Repository)`

---

## Files to CREATE

- `internal/features/data_export/loader/loader.go`:
  - `type Loader struct{ reader SourceReader; repo *repository.Repository; engine *validation.Engine; logger *slog.Logger; clock clock.Clock /* для тестов */ }`.
  - `func (l *Loader) Run(ctx context.Context, source string) (uuid.UUID, error)`:
    1. Repository.Loads.InsertRunning → `loadID`.
    2. Открыть транзакцию для всего load-а **нет** (слишком долгая) → используем короткие транзакции на batch UPSERT.
    3. Iter по каждой сущности (порядок: master сначала — products, category, location, supplier, ...; потом facts — receipt_line, ..., supplier_stock_snapshot).
    4. Каждая строка: map → engine.Check → critical в reject_log (counter `linesFailed`); иначе в staging-batch (UPSERT batch ~ 1000).
    5. После всех сущностей: проверить `linesFailed/linesTotal > 1%` → MarkFailed(`quality_threshold_exceeded`) → ошибка наружу.
    6. Открыть транзакцию → snapshot.Flip(loadID) → MarkCommitted → commit. Любая ошибка на этом шаге → MarkFailed.
    7. Метрики/логи (фактически метрики появятся в фазе 16, здесь — только логи).
- `internal/features/data_export/loader/types.go`:
  - `type EntityProgress struct{ Entity string; LinesTotal, LinesFailed int64 }`.
  - `type LoadResult struct{ LoadID uuid.UUID; Status models.LoadStatus; Entities []EntityProgress }`.
- `internal/features/data_export/loader/sequence.go`:
  - Константа порядка сущностей `EntityOrder = []string{...}`.
  - `func (l *Loader) loadEntity(ctx, loadID, entity string, fn func(ctx) error) error` — общая обёртка с подсчётом метрик.
- `internal/features/data_export/loader/loader_test.go` — unit:
  - `TestLoader_HappyPath` (mock reader + mock repo, fake clock) — проверяет последовательность вызовов: InsertRunning → SelectAll → UpsertAll → Flip → MarkCommitted.
  - `TestLoader_QualityThresholdExceeded` — 2% строк critical → MarkFailed(`quality_threshold_exceeded`), Flip не вызывается.
  - `TestLoader_ReaderError_MarksLoadFailed` — `SourceReader.ReadProducts` возвращает ошибку → MarkFailed(`erp_unavailable`).
  - `TestLoader_OptionalEntity_SupplierStockEmpty_OK` — пустой supplier_stock не приводит к failed.
  - `TestLoader_DuplicatePK_AddsToRejectLog` — engine ловит дубликат → reject_log.
  - `TestLoader_FlipFailure_MarksFailed` — repo.Flip падает → load failed, snapshot не меняется.
  - `TestLoader_EntityOrder_MasterBeforeFacts`.
- `internal/features/data_export/loader/mocks/mock_source_reader.go` — сгенерированный/руками mock (не используем mockery, чтобы не тащить лишнюю зависимость; пишем тривиальный handwritten mock).
- `internal/features/data_export/loader/mocks/mock_repository.go` — handwritten mock интерфейса `repositoryAPI` (см. ниже).

## Files to MODIFY

- `internal/features/data_export/repository/repository.go` — экспортировать неявный интерфейс `RepositoryAPI` (для mock-инжекции в loader). Альтернатива: loader принимает не `*Repository`, а интерфейс с нужным набором методов.
- `pkg/errorspkg/errors.go` — добавить sentinel `ErrQualityThresholdExceeded` (внутренний — не наружу), `ErrERPUnavailable` (внутренний).

## SQL/Migrations

— нет.

## Run after

```bash
make build
make test-unit
make lint
```

## Tests in this phase

- 7 unit-тестов в `loader_test.go` (см. список выше).
- Integration-тест loader-а с реальным PG откладываем в фазу 12 (там же, где scheduler).

## Definition of Done

- [ ] Loader выполняет master → facts последовательно.
- [ ] При quality threshold > 1% — load fail, snapshot не меняется.
- [ ] При ошибке reader — load fail.
- [ ] Snapshot flip атомарный (в транзакции).
- [ ] supplier_stock_snapshot опциональный — пустой не fail-ит.
- [ ] Все unit-тесты зелёные.
- [ ] `make build` / `make test-unit` / `make lint` зелёные.
- [ ] Коммит атомарный, сообщение `feat(data_export/loader): orchestration service ...`.
