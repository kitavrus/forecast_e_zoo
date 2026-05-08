// Package loader — phase 13: load-функции для оставшихся 7 entity
// (product_barcodes, promo, supply_plan, store_assortment, lifecycle_events,
// master_change_log, stock_movement). supplier_stock_snapshot обновлён в
// loader.go (его no-op версия заменена на real insert).
package loader

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/validation"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// loadProductBarcodes — UPSERT product_barcodes. FK products.product_id.
func (l *Loader) loadProductBarcodes(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadProductBarcodes(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.ProductBarcodeRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertProductBarcode(ctx, tx, row, loadID); err != nil {
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
		payload := map[string]any{"product_id": e.ProductID, "barcode": e.Barcode}
		violations := l.engine.Check("product_barcodes", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "product_barcodes", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}
		batch = append(batch, repository.ProductBarcodeRow{
			ProductID: e.ProductID,
			Barcode:   e.Barcode,
			IsPrimary: e.IsPrimary,
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

// loadPromo — UPSERT promo. FK location + products.
func (l *Loader) loadPromo(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadPromo(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.PromoRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertPromo(ctx, tx, row, loadID); err != nil {
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
		payload := map[string]any{"id": e.ID, "location_id": e.LocationID, "product_id": e.ProductID}
		violations := l.engine.Check("promo", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "promo", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}
		var payloadJSON []byte
		if e.Payload != nil {
			payloadJSON, _ = json.Marshal(e.Payload)
		}
		batch = append(batch, repository.PromoRow{
			ID:          e.ID,
			LocationID:  e.LocationID,
			ProductID:   e.ProductID,
			StartDate:   e.StartDate,
			EndDate:     e.EndDate,
			DiscountPct: e.DiscountPct,
			Payload:     payloadJSON,
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

// loadSupplyPlan — UPSERT supply_plan. FK location + products + supplier.
func (l *Loader) loadSupplyPlan(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadSupplyPlan(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.SupplyPlanRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertSupplyPlan(ctx, tx, row, loadID); err != nil {
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
		payload := map[string]any{"id": e.ID, "qty": e.Qty}
		violations := l.engine.Check("supply_plan", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "supply_plan", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}
		var payloadJSON []byte
		if e.Payload != nil {
			payloadJSON, _ = json.Marshal(e.Payload)
		}
		batch = append(batch, repository.SupplyPlanRow{
			ID:         e.ID,
			LocationID: e.LocationID,
			ProductID:  e.ProductID,
			SupplierID: e.SupplierID,
			PlanDate:   e.PlanDate,
			Qty:        e.Qty,
			Payload:    payloadJSON,
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

// loadStoreAssortment — UPSERT store_assortment. FK location + products.
func (l *Loader) loadStoreAssortment(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadStoreAssortment(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 500
	batch := make([]repository.StoreAssortmentRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if err := l.repo.UpsertStoreAssortment(ctx, tx, row, loadID); err != nil {
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
		payload := map[string]any{"location_id": e.LocationID, "product_id": e.ProductID}
		violations := l.engine.Check("store_assortment", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "store_assortment", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}
		batch = append(batch, repository.StoreAssortmentRow{
			LocationID: e.LocationID,
			ProductID:  e.ProductID,
			StartDate:  e.StartDate,
			EndDate:    e.EndDate,
			IsActive:   e.IsActive,
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

// loadLifecycleEvents — append-only INSERT lifecycle. FK location + products.
func (l *Loader) loadLifecycleEvents(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadStoreAssortmentLifecycleEvents(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 1000
	batch := make([]repository.LifecycleEventRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		if err := l.repo.InsertLifecycleEventBatch(ctx, tx, batch, loadID); err != nil {
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
		payload := map[string]any{"event_type": e.EventType, "event_date": e.EventDate}
		violations := l.engine.Check("store_assortment_lifecycle_events", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "store_assortment_lifecycle_events", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}
		var payloadJSON []byte
		if e.Payload != nil {
			payloadJSON, _ = json.Marshal(e.Payload)
		}
		batch = append(batch, repository.LifecycleEventRow{
			LocationID: e.LocationID,
			ProductID:  e.ProductID,
			EventType:  e.EventType,
			EventDate:  e.EventDate,
			Payload:    payloadJSON,
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

// loadMasterChangeLog — append-only журнал. FK нет (entity_pk хранится JSON).
func (l *Loader) loadMasterChangeLog(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadMasterChangeLog(ctx, l.since)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 1000
	batch := make([]repository.MasterChangeLogRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		if err := l.repo.InsertMasterChangeLogBatch(ctx, tx, batch, loadID); err != nil {
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
		payload := map[string]any{"entity": e.Entity, "field": e.Field}
		violations := l.engine.Check("master_change_log", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "master_change_log", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}
		entityPK, _ := json.Marshal(e.EntityPK)
		var oldVal, newVal []byte
		if e.OldValue != nil {
			oldVal, _ = json.Marshal(e.OldValue)
		}
		if e.NewValue != nil {
			newVal, _ = json.Marshal(e.NewValue)
		}
		batch = append(batch, repository.MasterChangeLogRow{
			Entity:    e.Entity,
			EntityPK:  entityPK,
			Field:     e.Field,
			OldValue:  oldVal,
			NewValue:  newVal,
			ChangedAt: e.ChangedAt,
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

// loadStockMovement — append-only INSERT (партиционированный fact).
// FK на products + location (app-level — partition tables не поддерживают FK).
func (l *Loader) loadStockMovement(ctx context.Context, loadID uuid.UUID, p *EntityProgress, state *validation.State) error {
	it, err := l.reader.ReadStockMovement(ctx, l.dateFrom, l.dateTo)
	if err != nil {
		return errorspkg.ErrERPUnavailable.Wrap(err)
	}
	defer func() { _ = it.Close() }()

	const batchSize = 1000
	batch := make([]repository.StockMovementRow, 0, batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		tx, err := l.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		if err := l.repo.InsertStockMovementBatch(ctx, tx, batch, loadID); err != nil {
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
		payload := map[string]any{"movement_type": e.MovementType, "qty": e.Qty}
		violations := l.engine.Check("stock_movement", payload, state)
		if hasCritical(violations) {
			p.LinesFailed++
			pl, _ := json.Marshal(e)
			ev, _ := json.Marshal(violations)
			_ = l.repo.InsertReject(ctx, repository.RejectInput{
				LoadID: loadID, Entity: "stock_movement", Severity: "error",
				Payload: pl, Errors: ev,
			})
			continue
		}
		var payloadJSON []byte
		if e.Payload != nil {
			payloadJSON, _ = json.Marshal(e.Payload)
		}
		batch = append(batch, repository.StockMovementRow{
			ID:           e.ID,
			EventDate:    e.EventDate,
			EventTime:    e.EventTime,
			LocationID:   e.LocationID,
			ProductID:    e.ProductID,
			MovementType: e.MovementType,
			Qty:          e.Qty,
			RefID:        e.RefID,
			Payload:      payloadJSON,
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
