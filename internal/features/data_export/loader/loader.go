package loader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/validation"
	"github.com/Kitavrus/e_zoo/internal/observability"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Clock — простая абстракция над временем (для тестов).
type Clock interface {
	Now() time.Time
}

type sysClock struct{}

func (sysClock) Now() time.Time { return time.Now() }

// SystemClock — production-реализация.
var SystemClock Clock = sysClock{}

// Loader — оркестратор суточного load-а.
type Loader struct {
	reader SourceReader
	repo   repository.LoaderAPI
	engine *validation.Engine
	logger *slog.Logger
	clock  Clock

	// since — окно для master-сущностей. По умолчанию — нулевая дата (full reload).
	since time.Time
	// dateFrom/dateTo — окно для facts. По умолчанию — последние 7 дней.
	dateFrom time.Time
	dateTo   time.Time
}

// Options — конструктор-опции.
type Options struct {
	Logger   *slog.Logger
	Clock    Clock
	Since    time.Time
	DateFrom time.Time
	DateTo   time.Time
}

// NewLoader создаёт Loader.
func NewLoader(reader SourceReader, repo repository.LoaderAPI, engine *validation.Engine, opts Options) *Loader {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Clock == nil {
		opts.Clock = SystemClock
	}
	if opts.DateTo.IsZero() {
		opts.DateTo = opts.Clock.Now()
	}
	if opts.DateFrom.IsZero() {
		opts.DateFrom = opts.DateTo.AddDate(0, 0, -7)
	}
	return &Loader{
		reader:   reader,
		repo:     repo,
		engine:   engine,
		logger:   opts.Logger,
		clock:    opts.Clock,
		since:    opts.Since,
		dateFrom: opts.DateFrom,
		dateTo:   opts.DateTo,
	}
}

// Run выполняет один load. Возвращает loadID и любую ошибку pipeline.
// Любая internal-ошибка → MarkFailed + возврат наружу.
//
// Метрики Prometheus:
//   - source_adapter_load_success_total{source}     — инкрементируется при успешном commit;
//   - source_adapter_load_failed_total{source,reason} — инкрементируется при любом MarkFailed;
//   - source_adapter_lines_total{entity}            — суммарные строки на сущность (после pipeline);
//   - source_adapter_lines_failed_total{entity,severity="critical"} — неуспешные (после pipeline).
func (l *Loader) Run(ctx context.Context, source string) (uuid.UUID, error) {
	load, err := l.repo.InsertRunning(ctx, source)
	if err != nil {
		// Не дошли даже до loadID — фиксируем как failed без load_id.
		observability.LoadFailedTotal.WithLabelValues(source, "insert_running").Inc()
		return uuid.Nil, fmt.Errorf("loader: insert running: %w", err)
	}
	loadID := load.ID
	l.logger.InfoContext(ctx, "loader.start", slog.String("load_id", loadID.String()), slog.String("source", source))

	state := validation.NewState(loadID.String())
	progress := make(map[string]*EntityProgress, len(EntityOrder))
	for _, e := range EntityOrder {
		progress[e] = &EntityProgress{Entity: e}
	}

	// 1. Master + facts pipeline.
	if err := l.runPipeline(ctx, loadID, progress, state); err != nil {
		reason := errReasonOf(err)
		_ = l.repo.MarkFailed(ctx, loadID, reason)
		observability.LoadFailedTotal.WithLabelValues(source, reason).Inc()
		l.logger.ErrorContext(ctx, "loader.failed", slog.String("load_id", loadID.String()), slog.Any("error", err))
		return loadID, err
	}

	// Per-entity счётчики (lines_processed / lines_failed) — после удачного pipeline.
	for _, p := range progress {
		if p.LinesTotal > 0 {
			observability.LinesProcessedTotal.WithLabelValues(p.Entity).Add(float64(p.LinesTotal))
		}
		if p.LinesFailed > 0 {
			observability.LinesFailedTotal.WithLabelValues(p.Entity, "critical").Add(float64(p.LinesFailed))
		}
	}

	// 2. Quality threshold.
	var totalLines, totalFailed int64
	for _, p := range progress {
		totalLines += p.LinesTotal
		totalFailed += p.LinesFailed
	}
	if totalLines > 0 && float64(totalFailed)/float64(totalLines) > QualityThresholdRatio {
		_ = l.repo.MarkFailed(ctx, loadID, errorspkg.ErrQualityThresholdExceeded.Code)
		observability.LoadFailedTotal.WithLabelValues(source, errorspkg.ErrQualityThresholdExceeded.Code).Inc()
		l.logger.WarnContext(ctx, "loader.quality_threshold_exceeded",
			slog.String("load_id", loadID.String()),
			slog.Int64("total", totalLines),
			slog.Int64("failed", totalFailed))
		return loadID, errorspkg.ErrQualityThresholdExceeded
	}

	// 3. Flip + commit (atomic).
	tx, err := l.repo.BeginTx(ctx)
	if err != nil {
		_ = l.repo.MarkFailed(ctx, loadID, "flip_begin_tx")
		observability.LoadFailedTotal.WithLabelValues(source, "flip_begin_tx").Inc()
		return loadID, fmt.Errorf("loader: begin tx: %w", err)
	}
	if _, err := l.repo.Flip(ctx, tx, loadID); err != nil {
		_ = tx.Rollback(ctx)
		_ = l.repo.MarkFailed(ctx, loadID, "flip_failed")
		observability.LoadFailedTotal.WithLabelValues(source, "flip_failed").Inc()
		return loadID, fmt.Errorf("loader: flip: %w", err)
	}
	statsJSON, _ := json.Marshal(progress)
	if err := l.repo.MarkCommitted(ctx, tx, loadID, totalLines, totalFailed, statsJSON); err != nil {
		_ = tx.Rollback(ctx)
		_ = l.repo.MarkFailed(ctx, loadID, "mark_committed_failed")
		observability.LoadFailedTotal.WithLabelValues(source, "mark_committed_failed").Inc()
		return loadID, fmt.Errorf("loader: mark committed: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		_ = l.repo.MarkFailed(ctx, loadID, "commit_failed")
		observability.LoadFailedTotal.WithLabelValues(source, "commit_failed").Inc()
		return loadID, fmt.Errorf("loader: commit: %w", err)
	}
	observability.LoadSuccessTotal.WithLabelValues(source).Inc()
	l.logger.InfoContext(ctx, "loader.committed",
		slog.String("load_id", loadID.String()),
		slog.Int64("lines_total", totalLines),
		slog.Int64("lines_failed", totalFailed))
	return loadID, nil
}

// errReasonOf — короткий код для loads.failure_reason из ошибки.
func errReasonOf(err error) string {
	if err == nil {
		return ""
	}
	var e *errorspkg.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "internal"
}

// runPipeline — последовательно прогоняет все сущности через ETL.
// Сейчас в стрейдж UPSERT'им только products и receipt_line (демо MVP);
// остальные сущности «считаем» — copy ERP→staging заглушаем (no-op),
// но прогоняем через validator и считаем счётчики.
//
// Полные UPSERT-методы для всех 16 сущностей — подключаются по мере
// расширения repository (вне этой фазы; 12+ методов).
//
// ВАЖНО (Issue #5 validation 2026-05-07): порядок строго master → facts.
// products зависит от category по FK (products_category_id_fkey), поэтому
// category ДОЛЖЕН вставляться раньше products. Раньше runPipeline стартовал
// с products → загрузка падала на FK violation.
//
// Текущий порядок отражает зависимости из EntityOrder:
//   1. category, supplier, location  — корневые master без внешних FK
//   2. products                       — зависит от category
//   3. receipt_line                   — fact, ссылается на products + location
//   4. supplier_stock_snapshot        — fact, optional (ERP может вернуть [])
//
// Остальные master/facts (product_barcodes, store_assortment, …) пока pass-through
// и подключаются по мере появления UPSERT-методов в repository (см. EntityOrder).
func (l *Loader) runPipeline(ctx context.Context, loadID uuid.UUID, progress map[string]*EntityProgress, state *validation.State) error {
	// --- 1. Master-сущности без FK (порядок внутри блока неважен) ---
	if err := l.loadGeneric(ctx, "category", loadID, progress, state); err != nil {
		return err
	}
	if err := l.loadGeneric(ctx, "supplier", loadID, progress, state); err != nil {
		return err
	}
	if err := l.loadGeneric(ctx, "location", loadID, progress, state); err != nil {
		return err
	}

	// --- 2. Master-сущности с FK на предыдущие ---
	// products зависит от category — обязательно ПОСЛЕ category.
	if err := l.loadProducts(ctx, loadID, progress["products"], state); err != nil {
		return err
	}

	// --- 2.1 Master с FK на supplier+products+location: supply_spec, order_rule ---
	// supply_spec ссылается на products + supplier.
	if err := l.loadSupplySpec(ctx, loadID, progress["supply_spec"], state); err != nil {
		return err
	}
	// order_rule ссылается на location, optional на products/category.
	if err := l.loadOrderRule(ctx, loadID, progress["order_rule"], state); err != nil {
		return err
	}

	// --- 3. Facts (после всех master) ---
	// receipt_line ссылается на products + location.
	if err := l.loadReceiptLine(ctx, loadID, progress["receipt_line"], state); err != nil {
		return err
	}
	// location_stock_snapshot ссылается на products + location.
	if err := l.loadLocationStockSnapshot(ctx, loadID, progress["location_stock_snapshot"], state); err != nil {
		return err
	}
	// supplier_stock_snapshot — optional: если ERP вернул [] — не считаем ошибкой.
	if err := l.loadSupplierStockSnapshot(ctx, loadID, progress["supplier_stock_snapshot"], state); err != nil {
		return err
	}

	return nil
}

// loadProducts — реальный пример: ERP→domain→engine.Check→UPSERT batch.
func (l *Loader) loadProducts(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadProducts(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.ProductRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertProduct(ctx, tx, row, loadID); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		erpP := it.Item()
		p.LinesTotal++

		// validator
		payload := map[string]any{"id": erpP.ID, "name": erpP.Name}
		violations := l.engine.Check("products", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			rejPayload, _ := json.Marshal(erpP)
			rejErrors, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "products", Severity: "error",
				Payload: rejPayload, Errors: rejErrors,
			})
			continue
		}

		attrs, _ := json.Marshal(erpP.Attributes)
		batch = append(batch, repository.ProductRow{
			ID: erpP.ID, SKU: erpP.SKU, Name: erpP.Name,
			CategoryID: erpP.CategoryID, Unit: erpP.Unit, PackSize: erpP.PackSize,
			IsActive: erpP.IsActive, Attributes: attrs,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// loadReceiptLine — пример обработки факта (партиционированного).
func (l *Loader) loadReceiptLine(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadReceiptLine(ctx, l.dateFrom, l.dateTo)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 1000
	batch := make([]repository.ReceiptLineRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		if err := l.repo.InsertReceiptLineBatch(ctx, tx, batch, loadID); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		e := it.Item()
		p.LinesTotal++

		payload := map[string]any{"qty": e.Qty, "event_time": e.EventTime}
		violations := l.engine.Check("receipt_line", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "receipt_line", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}

		details, _ := json.Marshal(e.Payload)
		batch = append(batch, repository.ReceiptLineRow{
			ID: e.ID, ReceiptID: e.ReceiptID, LocationID: e.LocationID, ProductID: e.ProductID,
			Qty: e.Qty, Price: e.Price, EventTime: e.EventTime, EventDate: e.EventDate, Payload: details,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// loadSupplierStockSnapshot — optional. Пустой ответ — OK, не валим load.
func (l *Loader) loadSupplierStockSnapshot(ctx context.Context, _ uuid.UUID, p *EntityProgress, _ *validation.State) error {
	it, err := l.reader.ReadSupplierStockSnapshot(ctx, l.dateFrom, l.dateTo)
	if err != nil {
		// Optional entity — лог, но не fail.
		l.logger.WarnContext(ctx, "loader.supplier_stock_unavailable", slog.Any("error", err))
		return nil
	}
	defer func() { _ = it.Close() }()
	for it.Next(ctx) {
		p.LinesTotal++
	}
	return nil
}

// loadGeneric — реальный UPSERT master-сущностей без сложной логики (category,
// supplier, location). Считает строки, прогоняет через engine, batched UPSERT.
//
// ВАЖНО (Issue #5 validation 2026-05-07; повторно 2026-05-08): раньше эта функция
// была pass-through (только считала Next()), из-за чего category/supplier/location
// никогда не попадали в БД, и products падал на products_category_id_fkey.
// Теперь каждый case делает реальный UPSERT в commit-tx так же, как loadProducts.
func (l *Loader) loadGeneric(ctx context.Context, entity string, loadID uuid.UUID, progress map[string]*EntityProgress, state *validation.State) error {
	p := progress[entity]
	switch entity {
	case "category":
		return l.loadCategory(ctx, loadID, p, state)
	case "location":
		return l.loadLocation(ctx, loadID, p, state)
	case "supplier":
		return l.loadSupplier(ctx, loadID, p, state)
	}
	return nil
}

// loadCategory — UPSERT category в БД. Обязательно ДО products (FK products.category_id).
func (l *Loader) loadCategory(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadCategory(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.CategoryRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertCategory(ctx, tx, row, loadID); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		c := it.Item()
		p.LinesTotal++
		payload := map[string]any{"id": c.ID, "name": c.Name}
		violations := l.engine.Check("category", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(c)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "category", Severity: "error", Payload: pl, Errors: ev,
			})
			continue
		}
		var pathPtr *string
		if c.Path != "" {
			pp := c.Path
			pathPtr = &pp
		}
		batch = append(batch, repository.CategoryRow{
			ID: c.ID, ParentID: c.ParentID, Name: c.Name, Path: pathPtr,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// loadSupplier — UPSERT supplier в БД. Обязательно ДО supplier_stock_snapshot/supply_spec.
func (l *Loader) loadSupplier(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadSupplier(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.SupplierRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertSupplier(ctx, tx, row, loadID); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		s := it.Item()
		p.LinesTotal++
		payload := map[string]any{"id": s.ID, "name": s.Name}
		violations := l.engine.Check("supplier", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(s)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "supplier", Severity: "error", Payload: pl, Errors: ev,
			})
			continue
		}
		batch = append(batch, repository.SupplierRow{
			ID: s.ID, Name: s.Name, INN: s.INN, KPP: s.KPP,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// loadLocation — UPSERT location в БД. Обязательно ДО receipt_line/stock_movement/
// location_stock_snapshot/store_assortment (NOT NULL FK).
func (l *Loader) loadLocation(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadLocation(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.LocationRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertLocation(ctx, tx, row, loadID); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		loc := it.Item()
		p.LinesTotal++
		payload := map[string]any{"id": loc.ID, "name": loc.Name, "type": loc.Type}
		violations := l.engine.Check("location", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(loc)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "location", Severity: "error", Payload: pl, Errors: ev,
			})
			continue
		}
		batch = append(batch, repository.LocationRow{
			ID: loc.ID, Type: loc.Type, Name: loc.Name,
			Region: loc.Region, Address: loc.Address, Lat: loc.Lat, Lon: loc.Lon,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// loadOrderRule — UPSERT order_rule в БД. Зависит от location (FK) и
// optional category/products. Пакетный UPSERT по аналогии с loadCategory.
func (l *Loader) loadOrderRule(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadOrderRule(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.OrderRuleRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertOrderRule(ctx, tx, row, loadID); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		o := it.Item()
		p.LinesTotal++
		// product_id / category_id оба могут быть NULL (location-wide rule;
		// прежний CHECK снят миграцией 0004).
		payload := map[string]any{"id": o.ID, "rule_type": o.RuleType}
		violations := l.engine.Check("order_rule", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(o)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "order_rule", Severity: "error", Payload: pl, Errors: ev,
			})
			continue
		}
		var payloadJSON []byte
		if o.Payload != nil {
			payloadJSON, _ = json.Marshal(o.Payload)
		}
		batch = append(batch, repository.OrderRuleRow{
			ID:         o.ID,
			LocationID: o.LocationID,
			ProductID:  o.ProductID,
			CategoryID: o.CategoryID,
			RuleType:   o.RuleType,
			Payload:    payloadJSON,
			ValidFrom:  o.ValidFrom,
			ValidTo:    o.ValidTo,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// loadSupplySpec — UPSERT supply_spec в БД. Зависит от products + supplier (FK).
func (l *Loader) loadSupplySpec(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadSupplySpec(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.SupplySpecRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertSupplySpec(ctx, tx, row, loadID); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		s := it.Item()
		p.LinesTotal++
		payload := map[string]any{"product_id": s.ProductID, "supplier_id": s.SupplierID}
		violations := l.engine.Check("supply_spec", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(s)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "supply_spec", Severity: "error", Payload: pl, Errors: ev,
			})
			continue
		}
		batch = append(batch, repository.SupplySpecRow{
			ProductID:    s.ProductID,
			SupplierID:   s.SupplierID,
			PackQty:      s.PackQty,
			LeadTimeDays: s.LeadTimeDays,
			MinOrderQty:  s.MinOrderQty,
			Multiple:     s.Multiple,
			ValidFrom:    s.ValidFrom,
			ValidTo:      s.ValidTo,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// loadLocationStockSnapshot — UPSERT location_stock_snapshot (партиционированный
// fact). Зависит от products + location (FK). Окно — dateFrom..dateTo.
func (l *Loader) loadLocationStockSnapshot(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadLocationStockSnapshot(ctx, l.dateFrom, l.dateTo)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 1000
	batch := make([]repository.LocationStockSnapshotRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertLocationStockSnapshot(ctx, tx, row, loadID); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for it.Next(ctx) {
		e := it.Item()
		p.LinesTotal++
		payload := map[string]any{"qty_on_hand": e.QtyOnHand, "event_date": e.EventDate}
		violations := l.engine.Check("location_stock_snapshot", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "location_stock_snapshot", Severity: "error", Payload: pl, Errors: ev,
			})
			continue
		}
		batch = append(batch, repository.LocationStockSnapshotRow{
			EventDate:   e.EventDate,
			LocationID:  e.LocationID,
			ProductID:   e.ProductID,
			QtyOnHand:   e.QtyOnHand,
			QtyReserved: e.QtyReserved,
			AsOf:        e.AsOf,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := it.Err(); err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	return flush()
}

// hasCritical возвращает true, если в violations есть severity=critical.
func hasCritical(vs []validation.Violation) bool {
	for _, v := range vs {
		if v.Severity == validation.SeverityCritical {
			return true
		}
	}
	return false
}

// _ keep imports stable.
var _ = models.LoadStatusRunning
