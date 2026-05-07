package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// ProductRow — внутренняя выборочная row для select_products (matches schema).
type ProductRow struct {
	ID         string
	SKU        string
	Name       string
	CategoryID *string
	Unit       string
	PackSize   *float64
	IsActive   bool
	Attributes []byte
	LoadID     *uuid.UUID
}

// SelectProducts — page с курсорной пагинацией по id.
func (r *Repository) SelectProducts(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]ProductRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_products"), loadID, afterPK, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := make([]ProductRow, 0, limit)
	for rows.Next() {
		var (
			p   ProductRow
			_ts any
		)
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.CategoryID, &p.Unit,
			&p.PackSize, &p.IsActive, &p.Attributes, &_ts, &p.LoadID); err != nil {
			return nil, mapError(err)
		}
		out = append(out, p)
	}
	return out, mapError(rows.Err())
}

// UpsertProduct вставляет/обновляет одну строку products.
// Используется loader-ом фазы 10.
func (r *Repository) UpsertProduct(ctx context.Context, tx pgx.Tx, p ProductRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertProduct requires tx")
	}
	if p.Attributes == nil {
		p.Attributes = []byte("{}")
	}
	const sql = `
INSERT INTO products (id, sku, name, category_id, unit, pack_size, is_active, attributes, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
ON CONFLICT (id) DO UPDATE SET
    sku        = EXCLUDED.sku,
    name       = EXCLUDED.name,
    category_id= EXCLUDED.category_id,
    unit       = EXCLUDED.unit,
    pack_size  = EXCLUDED.pack_size,
    is_active  = EXCLUDED.is_active,
    attributes = EXCLUDED.attributes,
    updated_at = now(),
    load_id    = EXCLUDED.load_id;
`
	_, err := tx.Exec(ctx, sql,
		p.ID, p.SKU, p.Name, p.CategoryID, p.Unit,
		p.PackSize, p.IsActive, p.Attributes, loadID)
	return mapError(err)
}

// CategoryRow — строка master-таблицы category.
// Используется loader-ом для UPSERT и (опционально) для select-выборок.
type CategoryRow struct {
	ID       string
	ParentID *string
	Name     string
	Path     *string // ltree::text — для in-memory stub обычно nil/пусто.
}

// UpsertCategory вставляет/обновляет одну строку category.
// Категории должны вставляться ДО products — иначе FK products_category_id_fkey.
func (r *Repository) UpsertCategory(ctx context.Context, tx pgx.Tx, c CategoryRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertCategory requires tx")
	}
	const sql = `
INSERT INTO category (id, parent_id, name, path, load_id)
VALUES ($1, $2, $3, NULLIF($4, '')::ltree, $5)
ON CONFLICT (id) DO UPDATE SET
    parent_id  = EXCLUDED.parent_id,
    name       = EXCLUDED.name,
    path       = EXCLUDED.path,
    updated_at = now(),
    load_id    = EXCLUDED.load_id;
`
	var pathStr string
	if c.Path != nil {
		pathStr = *c.Path
	}
	_, err := tx.Exec(ctx, sql, c.ID, c.ParentID, c.Name, pathStr, loadID)
	return mapError(err)
}

// SupplierRow — строка master-таблицы supplier.
type SupplierRow struct {
	ID   string
	Name string
	INN  *string
	KPP  *string
}

// UpsertSupplier вставляет/обновляет одну строку supplier.
// Должен вызываться до фактов с supplier_id (supplier_stock_snapshot, supply_spec).
func (r *Repository) UpsertSupplier(ctx context.Context, tx pgx.Tx, s SupplierRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertSupplier requires tx")
	}
	const sql = `
INSERT INTO supplier (id, name, inn, kpp, load_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET
    name       = EXCLUDED.name,
    inn        = EXCLUDED.inn,
    kpp        = EXCLUDED.kpp,
    updated_at = now(),
    load_id    = EXCLUDED.load_id;
`
	_, err := tx.Exec(ctx, sql, s.ID, s.Name, s.INN, s.KPP, loadID)
	return mapError(err)
}

// LocationRow — строка master-таблицы location.
type LocationRow struct {
	ID      string
	Type    string
	Name    string
	Region  *string
	Address *string
	Lat     *float64
	Lon     *float64
}

// UpsertLocation вставляет/обновляет одну строку location.
// Должен вызываться до фактов с location_id (receipt_line, stock_movement,
// location_stock_snapshot, store_assortment).
func (r *Repository) UpsertLocation(ctx context.Context, tx pgx.Tx, l LocationRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("UpsertLocation requires tx")
	}
	const sql = `
INSERT INTO location (id, type, name, region, address, lat, lon, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    type       = EXCLUDED.type,
    name       = EXCLUDED.name,
    region     = EXCLUDED.region,
    address    = EXCLUDED.address,
    lat        = EXCLUDED.lat,
    lon        = EXCLUDED.lon,
    updated_at = now(),
    load_id    = EXCLUDED.load_id;
`
	_, err := tx.Exec(ctx, sql, l.ID, l.Type, l.Name, l.Region, l.Address, l.Lat, l.Lon, loadID)
	return mapError(err)
}
