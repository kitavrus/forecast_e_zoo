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
func (l *Loader) Run(ctx context.Context, source string) (uuid.UUID, error) {
	load, err := l.repo.InsertRunning(ctx, source)
	if err != nil {
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
		// Mark failed + return.
		_ = l.repo.MarkFailed(ctx, loadID, errReasonOf(err))
		l.logger.ErrorContext(ctx, "loader.failed", slog.String("load_id", loadID.String()), slog.Any("error", err))
		return loadID, err
	}

	// 2. Quality threshold.
	var totalLines, totalFailed int64
	for _, p := range progress {
		totalLines += p.LinesTotal
		totalFailed += p.LinesFailed
	}
	if totalLines > 0 && float64(totalFailed)/float64(totalLines) > QualityThresholdRatio {
		_ = l.repo.MarkFailed(ctx, loadID, errorspkg.ErrQualityThresholdExceeded.Code)
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
		return loadID, fmt.Errorf("loader: begin tx: %w", err)
	}
	if _, err := l.repo.Flip(ctx, tx, loadID); err != nil {
		_ = tx.Rollback(ctx)
		_ = l.repo.MarkFailed(ctx, loadID, "flip_failed")
		return loadID, fmt.Errorf("loader: flip: %w", err)
	}
	statsJSON, _ := json.Marshal(progress)
	if err := l.repo.MarkCommitted(ctx, tx, loadID, totalLines, totalFailed, statsJSON); err != nil {
		_ = tx.Rollback(ctx)
		_ = l.repo.MarkFailed(ctx, loadID, "mark_committed_failed")
		return loadID, fmt.Errorf("loader: mark committed: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		_ = l.repo.MarkFailed(ctx, loadID, "commit_failed")
		return loadID, fmt.Errorf("loader: commit: %w", err)
	}
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
func (l *Loader) runPipeline(ctx context.Context, loadID uuid.UUID, progress map[string]*EntityProgress, state *validation.State) error {
	// products (с реальным UPSERT)
	if err := l.loadProducts(ctx, loadID, progress["products"], state); err != nil {
		return err
	}
	// receipt_line (с реальным batch INSERT в партиции)
	if err := l.loadReceiptLine(ctx, loadID, progress["receipt_line"], state); err != nil {
		return err
	}
	// supplier_stock_snapshot — optional: если ERP вернул [] — не считаем ошибкой.
	if err := l.loadSupplierStockSnapshot(ctx, loadID, progress["supplier_stock_snapshot"], state); err != nil {
		return err
	}
	// для остальных сущностей — pass-through через iterator + validator (без UPSERT в БД).
	if err := l.loadGeneric(ctx, "category", loadID, progress, state); err != nil {
		return err
	}
	if err := l.loadGeneric(ctx, "location", loadID, progress, state); err != nil {
		return err
	}
	if err := l.loadGeneric(ctx, "supplier", loadID, progress, state); err != nil {
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

// loadGeneric — pass-through: считаем строки, прогоняем через engine, UPSERT нет.
func (l *Loader) loadGeneric(ctx context.Context, entity string, loadID uuid.UUID, progress map[string]*EntityProgress, state *validation.State) error {
	p := progress[entity]
	switch entity {
	case "category":
		it, err := l.reader.ReadCategory(ctx, l.since)
		if err != nil {
			return errorspkg.ErrERPUnavailable.Wrap(err)
		}
		defer func() { _ = it.Close() }()
		for it.Next(ctx) {
			p.LinesTotal++
		}
		return it.Err()
	case "location":
		it, err := l.reader.ReadLocation(ctx, l.since)
		if err != nil {
			return errorspkg.ErrERPUnavailable.Wrap(err)
		}
		defer func() { _ = it.Close() }()
		for it.Next(ctx) {
			it2 := it.Item()
			p.LinesTotal++
			payload := map[string]any{"id": it2.ID, "name": it2.Name, "type": it2.Type}
			violations := l.engine.Check("location", payload, state)
			if hasCritical(violations) {
				p.LinesFailed++
				pl, _ := json.Marshal(it2)
				ev, _ := json.Marshal(violations)
				_ = l.repo.InsertReject(ctx, repository.RejectInput{
					LoadID: loadID, Entity: "location", Severity: "error", Payload: pl, Errors: ev,
				})
			}
		}
		return it.Err()
	case "supplier":
		it, err := l.reader.ReadSupplier(ctx, l.since)
		if err != nil {
			return errorspkg.ErrERPUnavailable.Wrap(err)
		}
		defer func() { _ = it.Close() }()
		for it.Next(ctx) {
			p.LinesTotal++
		}
		return it.Err()
	}
	return nil
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
