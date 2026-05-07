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
