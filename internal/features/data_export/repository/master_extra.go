// Package repository — расширение master.go для дополнительных master-сущностей,
// нужных ETL Module 2 (mart_calculation_input): order_rule, supply_spec.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// OrderRuleRow — строка master-таблицы order_rule.
//
// CHECK constraint в БД: (product_id IS NOT NULL OR category_id IS NOT NULL).
// Хотя бы одно из двух поле обязано быть set — иначе UPSERT упадёт.
// Loader делает defensive check перед добавлением в batch.
type OrderRuleRow struct {
	ID         string
	LocationID string
	ProductID  *string
	CategoryID *string
	RuleType   string
	Payload    []byte // jsonb
	ValidFrom  time.Time
	ValidTo    *time.Time
}

// UpsertOrderRule вставляет/обновляет одну строку order_rule.
// Должен вызываться после category, supplier, location, products
// (FK на location/products/category).
func (r *Repository) UpsertOrderRule(ctx context.Context, tx pgx.Tx, row OrderRuleRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertOrderRule requires tx")
	}
	if row.Payload == nil {
		row.Payload = []byte("{}")
	}
	_, err := tx.Exec(ctx, queries.Get("upsert_order_rule"),
		row.ID, row.LocationID, row.ProductID, row.CategoryID, row.RuleType,
		row.Payload, row.ValidFrom, row.ValidTo, loadID)
	return mapError(err)
}

// SupplySpecRow — строка master-таблицы supply_spec.
// Composite PK: (product_id, supplier_id, valid_from).
type SupplySpecRow struct {
	ProductID    string
	SupplierID   string
	PackQty      *float64
	LeadTimeDays *int
	MinOrderQty  *float64
	Multiple     *float64
	ValidFrom    time.Time
	ValidTo      *time.Time
}

// UpsertSupplySpec вставляет/обновляет одну строку supply_spec.
// Должен вызываться после products + supplier (FK на products/supplier).
func (r *Repository) UpsertSupplySpec(ctx context.Context, tx pgx.Tx, row SupplySpecRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertSupplySpec requires tx")
	}
	_, err := tx.Exec(ctx, queries.Get("upsert_supply_spec"),
		row.ProductID, row.SupplierID, row.PackQty, row.LeadTimeDays,
		row.MinOrderQty, row.Multiple, row.ValidFrom, row.ValidTo, loadID)
	return mapError(err)
}

// LocationStockSnapshotRow — строка fact-таблицы location_stock_snapshot
// (партиционирована по event_date).
//
// Composite PK: (event_date, location_id, product_id).
type LocationStockSnapshotRow struct {
	EventDate   time.Time
	LocationID  string
	ProductID   string
	QtyOnHand   float64
	QtyReserved float64
	AsOf        time.Time
	LoadID      *uuid.UUID
}

// UpsertLocationStockSnapshot вставляет/обновляет одну строку location_stock_snapshot.
// Loader предварительно вставляет location и products (FK).
func (r *Repository) UpsertLocationStockSnapshot(ctx context.Context, tx pgx.Tx, row LocationStockSnapshotRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertLocationStockSnapshot requires tx")
	}
	_, err := tx.Exec(ctx, queries.Get("upsert_location_stock_snapshot"),
		row.EventDate, row.LocationID, row.ProductID,
		row.QtyOnHand, row.QtyReserved, row.AsOf, loadID)
	return mapError(err)
}

// SelectCategory — page с курсорной пагинацией по id.
func (r *Repository) SelectCategory(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]CategoryRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_category"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]CategoryRow, 0, limit)
	for rows.Next() {
		var (
			c        CategoryRow
			ts       any
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Path, &ts, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		out = append(out, c)
	}
	return out, mapError(rows.Err())
}

// SelectSupplier — page с курсорной пагинацией по id.
func (r *Repository) SelectSupplier(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]SupplierRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_supplier"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]SupplierRow, 0, limit)
	for rows.Next() {
		var (
			s        SupplierRow
			ts       any
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&s.ID, &s.Name, &s.INN, &s.KPP, &ts, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		out = append(out, s)
	}
	return out, mapError(rows.Err())
}

// SelectLocation — page с курсорной пагинацией по id.
func (r *Repository) SelectLocation(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]LocationRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_location"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]LocationRow, 0, limit)
	for rows.Next() {
		var (
			l        LocationRow
			ts       any
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(&l.ID, &l.Type, &l.Name, &l.Region, &l.Address, &l.Lat, &l.Lon, &ts, &loadIDDB); err != nil {
			return nil, mapError(err)
		}
		out = append(out, l)
	}
	return out, mapError(rows.Err())
}

// OrderRuleFannedRow — fanned-out строка order_rule с обязательными
// product_id / location_id для ETL transform.
type OrderRuleFannedRow struct {
	RuleID     string
	LocationID string
	ProductID  string
	RuleType   string
	Payload    []byte
	ValidFrom  time.Time
	ValidTo    *time.Time
}

// SelectOrderRuleFanout — page fanned-out order_rule по продуктам/категориям.
// Используется handler-ом /v1/order_rule.
func (r *Repository) SelectOrderRuleFanout(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]OrderRuleFannedRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_order_rule_fanout"),
		loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]OrderRuleFannedRow, 0, limit)
	for rows.Next() {
		var (
			row     OrderRuleFannedRow
			payload []byte
		)
		if err := rows.Scan(&row.RuleID, &row.LocationID, &row.ProductID,
			&row.RuleType, &payload, &row.ValidFrom, &row.ValidTo); err != nil {
			return nil, mapError(err)
		}
		row.Payload = payload
		out = append(out, row)
	}
	return out, mapError(rows.Err())
}

// SupplySpecFannedRow — fanned-out supply_spec с location_id (CROSS JOIN
// с location). ETL ожидает composite (supplier_id, product_id, location_id).
type SupplySpecFannedRow struct {
	ProductID    string
	SupplierID   string
	LocationID   string
	PackQty      *float64
	LeadTimeDays *int
	MinOrderQty  *float64
	Multiple     *float64
	ValidFrom    time.Time
	ValidTo      *time.Time
}

// SelectSupplySpecFanout — page fanned-out supply_spec по локациям.
// Используется handler-ом /v1/supply_spec.
func (r *Repository) SelectSupplySpecFanout(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]SupplySpecFannedRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_supply_spec_fanout"),
		loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]SupplySpecFannedRow, 0, limit)
	for rows.Next() {
		var ss SupplySpecFannedRow
		if err := rows.Scan(&ss.ProductID, &ss.SupplierID, &ss.LocationID,
			&ss.PackQty, &ss.LeadTimeDays, &ss.MinOrderQty, &ss.Multiple,
			&ss.ValidFrom, &ss.ValidTo); err != nil {
			return nil, mapError(err)
		}
		out = append(out, ss)
	}
	return out, mapError(rows.Err())
}

// SelectOrderRule — page с курсорной пагинацией по id.
func (r *Repository) SelectOrderRule(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]OrderRuleRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_order_rule"),
		loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]OrderRuleRow, 0, limit)
	for rows.Next() {
		var (
			or       OrderRuleRow
			payload  []byte
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(
			&or.ID, &or.LocationID, &or.ProductID, &or.CategoryID,
			&or.RuleType, &payload, &or.ValidFrom, &or.ValidTo, &loadIDDB,
		); err != nil {
			return nil, mapError(err)
		}
		or.Payload = payload
		out = append(out, or)
	}
	return out, mapError(rows.Err())
}

// SelectSupplySpec — page с курсорной пагинацией по composite PK.
func (r *Repository) SelectSupplySpec(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]SupplySpecRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_supply_spec"),
		loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]SupplySpecRow, 0, limit)
	for rows.Next() {
		var (
			ss       SupplySpecRow
			loadIDDB *uuid.UUID
		)
		if err := rows.Scan(
			&ss.ProductID, &ss.SupplierID, &ss.PackQty, &ss.LeadTimeDays,
			&ss.MinOrderQty, &ss.Multiple, &ss.ValidFrom, &ss.ValidTo, &loadIDDB,
		); err != nil {
			return nil, mapError(err)
		}
		out = append(out, ss)
	}
	return out, mapError(rows.Err())
}

// SelectLocationStockSnapshot — page с курсорной пагинацией по composite PK +
// фильтром event_date BETWEEN dateFrom AND dateTo (партиционирование).
func (r *Repository) SelectLocationStockSnapshot(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]LocationStockSnapshotRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_location_stock_snapshot"),
		loadID, afterPK, limit, dateFrom, dateTo)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]LocationStockSnapshotRow, 0, limit)
	for rows.Next() {
		var lss LocationStockSnapshotRow
		if err := rows.Scan(
			&lss.EventDate, &lss.LocationID, &lss.ProductID,
			&lss.QtyOnHand, &lss.QtyReserved, &lss.AsOf, &lss.LoadID,
		); err != nil {
			return nil, mapError(err)
		}
		out = append(out, lss)
	}
	return out, mapError(rows.Err())
}
