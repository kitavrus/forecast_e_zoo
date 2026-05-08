// Package repository — расширение master_extra.go: репозиторные методы для
// 8 master/facts сущностей, реализуемых в фазе 13 (полный 16/16 ingest):
//
//   - product_barcodes (master, UPSERT)
//   - promo (master, UPSERT)
//   - supply_plan (master, UPSERT)
//   - store_assortment (master, UPSERT)
//   - master_change_log (append-only batch INSERT)
//   - store_assortment_lifecycle_events (append-only batch INSERT)
//   - stock_movement (partitioned facts, INSERT)
//   - supplier_stock_snapshot (partitioned facts, INSERT)
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// =====================================================================
// Master rows (UPSERT).
// =====================================================================

// ProductBarcodeRow — строка master-таблицы product_barcodes (1:N к products).
// Composite PK = (product_id, barcode). load_id добавлен миграцией 0005.
type ProductBarcodeRow struct {
	ProductID string
	Barcode   string
	IsPrimary bool
}

// UpsertProductBarcode вставляет/обновляет один barcode.
// Должен вызываться после products (FK product_id).
func (r *Repository) UpsertProductBarcode(ctx context.Context, tx pgx.Tx, row ProductBarcodeRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertProductBarcode requires tx")
	}
	_, err := tx.Exec(ctx, queries.Get("upsert_product_barcodes"),
		row.ProductID, row.Barcode, row.IsPrimary, loadID)
	return mapError(err)
}

// PromoRow — строка master-таблицы promo. PK = id (text).
type PromoRow struct {
	ID          string
	LocationID  string
	ProductID   string
	StartDate   time.Time
	EndDate     time.Time
	DiscountPct *float64
	Payload     []byte // jsonb
}

// UpsertPromo вставляет/обновляет одну акцию.
// Должен вызываться после location + products (FK).
func (r *Repository) UpsertPromo(ctx context.Context, tx pgx.Tx, row PromoRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertPromo requires tx")
	}
	if row.Payload == nil {
		row.Payload = []byte("{}")
	}
	_, err := tx.Exec(ctx, queries.Get("upsert_promo"),
		row.ID, row.LocationID, row.ProductID, row.StartDate, row.EndDate,
		row.DiscountPct, row.Payload, loadID)
	return mapError(err)
}

// SupplyPlanRow — строка master-таблицы supply_plan. PK = id (text).
type SupplyPlanRow struct {
	ID         string
	LocationID string
	ProductID  string
	SupplierID string
	PlanDate   time.Time
	Qty        float64
	Payload    []byte // jsonb
}

// UpsertSupplyPlan вставляет/обновляет одну запись плана поставок.
// Должен вызываться после location + products + supplier (FK).
func (r *Repository) UpsertSupplyPlan(ctx context.Context, tx pgx.Tx, row SupplyPlanRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertSupplyPlan requires tx")
	}
	if row.Payload == nil {
		row.Payload = []byte("{}")
	}
	_, err := tx.Exec(ctx, queries.Get("upsert_supply_plan"),
		row.ID, row.LocationID, row.ProductID, row.SupplierID, row.PlanDate,
		row.Qty, row.Payload, loadID)
	return mapError(err)
}

// StoreAssortmentRow — строка master-таблицы store_assortment.
// Composite PK = (location_id, product_id).
type StoreAssortmentRow struct {
	LocationID string
	ProductID  string
	StartDate  time.Time
	EndDate    *time.Time
	IsActive   bool
}

// UpsertStoreAssortment вставляет/обновляет одну строку ассортимента.
// Должен вызываться после location + products (FK).
func (r *Repository) UpsertStoreAssortment(ctx context.Context, tx pgx.Tx, row StoreAssortmentRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertStoreAssortment requires tx")
	}
	_, err := tx.Exec(ctx, queries.Get("upsert_store_assortment"),
		row.LocationID, row.ProductID, row.StartDate, row.EndDate,
		row.IsActive, loadID)
	return mapError(err)
}

// =====================================================================
// Append-only inserts.
// =====================================================================

// LifecycleEventRow — append-only строка store_assortment_lifecycle_events.
// id (bigserial) генерируется БД.
type LifecycleEventRow struct {
	LocationID string
	ProductID  string
	EventType  string // 'start' | 'stop' | 'promo' (CHECK constraint в схеме)
	EventDate  time.Time
	Payload    []byte // jsonb
}

// InsertLifecycleEventBatch — batch INSERT lifecycle events в одну транзакцию.
// Ничего не делает при пустом batch.
func (r *Repository) InsertLifecycleEventBatch(ctx context.Context, tx pgx.Tx, batch []LifecycleEventRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("InsertLifecycleEventBatch requires tx")
	}
	if len(batch) == 0 {
		return nil
	}
	q := queries.Get("insert_store_assortment_lifecycle_events")
	for _, row := range batch {
		if row.Payload == nil {
			row.Payload = []byte("{}")
		}
		if _, err := tx.Exec(ctx, q,
			row.LocationID, row.ProductID, row.EventType, row.EventDate,
			row.Payload, loadID,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// MasterChangeLogRow — append-only строка master_change_log.
// id (bigserial) генерируется БД.
type MasterChangeLogRow struct {
	Entity    string
	EntityPK  []byte // jsonb
	Field     string
	OldValue  []byte // jsonb (nullable, "null" if absent)
	NewValue  []byte // jsonb (nullable, "null" if absent)
	ChangedAt time.Time
}

// InsertMasterChangeLogBatch — batch INSERT master_change_log в одну транзакцию.
// Ничего не делает при пустом batch.
func (r *Repository) InsertMasterChangeLogBatch(ctx context.Context, tx pgx.Tx, batch []MasterChangeLogRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("InsertMasterChangeLogBatch requires tx")
	}
	if len(batch) == 0 {
		return nil
	}
	q := queries.Get("insert_master_change_log")
	for _, row := range batch {
		if row.EntityPK == nil {
			row.EntityPK = []byte("{}")
		}
		oldV := nullableJSON(row.OldValue)
		newV := nullableJSON(row.NewValue)
		if _, err := tx.Exec(ctx, q,
			row.Entity, row.EntityPK, row.Field, oldV, newV,
			nullableTime(row.ChangedAt), loadID,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// =====================================================================
// Partitioned facts (INSERT).
// =====================================================================

// StockMovementRow — fact-row stock_movement (партиционировано по event_date).
// Composite PK = (event_date, id).
type StockMovementRow struct {
	ID           int64
	EventDate    time.Time
	EventTime    time.Time
	LocationID   string
	ProductID    string
	MovementType string
	Qty          float64
	RefID        *string
	Payload      []byte // jsonb
}

// InsertStockMovementBatch — batch INSERT движений остатков.
func (r *Repository) InsertStockMovementBatch(ctx context.Context, tx pgx.Tx, batch []StockMovementRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("InsertStockMovementBatch requires tx")
	}
	if len(batch) == 0 {
		return nil
	}
	q := queries.Get("insert_stock_movement")
	for _, row := range batch {
		if row.Payload == nil {
			row.Payload = []byte("{}")
		}
		if _, err := tx.Exec(ctx, q,
			row.ID, row.EventDate, row.EventTime, row.LocationID, row.ProductID,
			row.MovementType, row.Qty, row.RefID, row.Payload, loadID,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// SupplierStockSnapshotRow — fact-row supplier_stock_snapshot
// (партиционировано по event_date).
// Composite PK = (event_date, supplier_id, product_id).
type SupplierStockSnapshotRow struct {
	EventDate    time.Time
	SupplierID   string
	ProductID    string
	QtyAvailable float64
	AsOf         time.Time
}

// InsertSupplierStockSnapshotBatch — batch INSERT (с idempotent ON CONFLICT)
// supplier stock snapshot. Должен вызываться после supplier (FK).
func (r *Repository) InsertSupplierStockSnapshotBatch(ctx context.Context, tx pgx.Tx, batch []SupplierStockSnapshotRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("InsertSupplierStockSnapshotBatch requires tx")
	}
	if len(batch) == 0 {
		return nil
	}
	q := queries.Get("insert_supplier_stock_snapshot")
	for _, row := range batch {
		if _, err := tx.Exec(ctx, q,
			row.EventDate, row.SupplierID, row.ProductID,
			row.QtyAvailable, row.AsOf, loadID,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// =====================================================================
// SELECT methods (cursor pagination для handlers).
// =====================================================================

// SelectProductBarcodes — page по composite cursor "<product_id>|<barcode>".
func (r *Repository) SelectProductBarcodes(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]ProductBarcodeRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_product_barcodes"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]ProductBarcodeRow, 0, limit)
	for rows.Next() {
		var (
			pb       ProductBarcodeRow
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&pb.ProductID, &pb.Barcode, &pb.IsPrimary, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		out = append(out, pb)
	}
	return out, mapError(rows.Err())
}

// SelectPromo — page по cursor по id.
func (r *Repository) SelectPromo(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]PromoRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_promo"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]PromoRow, 0, limit)
	for rows.Next() {
		var (
			p        PromoRow
			payload  []byte
			ts       any
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&p.ID, &p.LocationID, &p.ProductID, &p.StartDate, &p.EndDate,
			&p.DiscountPct, &payload, &ts, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		p.Payload = payload
		out = append(out, p)
	}
	return out, mapError(rows.Err())
}

// SelectSupplyPlan — page по cursor по id.
func (r *Repository) SelectSupplyPlan(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]SupplyPlanRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_supply_plan"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]SupplyPlanRow, 0, limit)
	for rows.Next() {
		var (
			sp       SupplyPlanRow
			payload  []byte
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&sp.ID, &sp.LocationID, &sp.ProductID, &sp.SupplierID,
			&sp.PlanDate, &sp.Qty, &payload, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		sp.Payload = payload
		out = append(out, sp)
	}
	return out, mapError(rows.Err())
}

// SelectStoreAssortment — page по composite cursor "<location_id>|<product_id>|<start_date>".
func (r *Repository) SelectStoreAssortment(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]StoreAssortmentRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_store_assortment"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]StoreAssortmentRow, 0, limit)
	for rows.Next() {
		var (
			sa       StoreAssortmentRow
			ts       any
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&sa.LocationID, &sa.ProductID, &sa.StartDate, &sa.EndDate,
			&sa.IsActive, &ts, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		out = append(out, sa)
	}
	return out, mapError(rows.Err())
}

// LifecycleEventReadRow — выборочная row для select_store_assortment_lifecycle_events.
type LifecycleEventReadRow struct {
	ID         int64
	LocationID string
	ProductID  string
	EventType  string
	EventDate  time.Time
	Payload    []byte
	LoadID     *uuid.UUID
}

// SelectLifecycleEvents — page по cursor по id (bigint).
func (r *Repository) SelectLifecycleEvents(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]LifecycleEventReadRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	if afterPK == "" {
		afterPK = "0"
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_store_assortment_lifecycle_events"),
		loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]LifecycleEventReadRow, 0, limit)
	for rows.Next() {
		var (
			le      LifecycleEventReadRow
			payload []byte
		)
		if err := rows.Scan(&le.ID, &le.LocationID, &le.ProductID, &le.EventType,
			&le.EventDate, &payload, &le.LoadID); err != nil {
			return nil, mapError(err)
		}
		le.Payload = payload
		out = append(out, le)
	}
	return out, mapError(rows.Err())
}

// SelectStockMovement — page с фильтром event_date BETWEEN.
func (r *Repository) SelectStockMovement(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]StockMovementRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_stock_movement"),
		loadID, afterPK, limit, dateFrom, dateTo)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]StockMovementRow, 0, limit)
	for rows.Next() {
		var (
			sm       StockMovementRow
			payload  []byte
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&sm.ID, &sm.EventDate, &sm.EventTime, &sm.LocationID, &sm.ProductID,
			&sm.MovementType, &sm.Qty, &sm.RefID, &payload, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		sm.Payload = payload
		out = append(out, sm)
	}
	return out, mapError(rows.Err())
}

// SelectSupplierStockSnapshot — page с фильтром event_date BETWEEN.
func (r *Repository) SelectSupplierStockSnapshot(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]SupplierStockSnapshotRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_supplier_stock_snapshot"),
		loadID, afterPK, limit, dateFrom, dateTo)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]SupplierStockSnapshotRow, 0, limit)
	for rows.Next() {
		var (
			sss      SupplierStockSnapshotRow
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&sss.EventDate, &sss.SupplierID, &sss.ProductID,
			&sss.QtyAvailable, &sss.AsOf, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		out = append(out, sss)
	}
	return out, mapError(rows.Err())
}

// MasterChangeLogReadRow — алиас к существующему ChangeLogRow.
// Сохранён для устойчивого имени в публичном handler API.
type MasterChangeLogReadRow = ChangeLogRow

// SelectMasterChangeLog — page по cursor по id.
// Алиас к существующему SelectChangeLog с публичным именем согласованным
// с handler-эндпоинтом /v1/master_change_log.
func (r *Repository) SelectMasterChangeLog(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]MasterChangeLogReadRow, error) {
	rows, err := r.SelectChangeLog(ctx, loadID, afterPK, limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// nullableJSON — превращает empty/nil [] в pgx-NULL для jsonb колонок.
func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
