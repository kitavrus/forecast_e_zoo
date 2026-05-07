package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/sqls/queries"
)

// GetSupplierMaster — читает supplier из marts.mart_master_current.
// Если строка отсутствует — возвращает models.SupplierMaster с HasMartRow=false (zero defaults).
func (r *Repository) GetSupplierMaster(
	ctx context.Context, supplierID string,
) (models.SupplierMaster, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_supplier_master"), supplierID)
	var (
		currency  string
		leadTime  int
		defPrice  *float64
	)
	err := row.Scan(&currency, &leadTime, &defPrice)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.SupplierMaster{SupplierID: supplierID, HasMartRow: false}, nil
		}
		return models.SupplierMaster{}, fmt.Errorf("orders: get supplier master: %w", err)
	}
	return models.SupplierMaster{
		SupplierID:       supplierID,
		Currency:         currency,
		LeadTimeDays:     leadTime,
		DefaultUnitPrice: defPrice,
		HasMartRow:       true,
	}, nil
}

// GetProductMaster — читает product unit_price из marts.mart_master_current.
// Отсутствие строки — не ошибка.
func (r *Repository) GetProductMaster(
	ctx context.Context, productID string,
) (models.ProductMaster, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_product_master"), productID)
	var price *float64
	err := row.Scan(&price)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ProductMaster{ProductID: productID, HasMartRow: false}, nil
		}
		return models.ProductMaster{}, fmt.Errorf("orders: get product master: %w", err)
	}
	return models.ProductMaster{
		ProductID:  productID,
		UnitPrice:  price,
		HasMartRow: true,
	}, nil
}
