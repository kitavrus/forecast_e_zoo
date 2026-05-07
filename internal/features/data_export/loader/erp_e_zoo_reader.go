package loader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErpEZooReader — in-memory MVP-реализация SourceReader. Загружает все
// fixtures с диска при New(); каждый Read* метод применяет since/event_date
// фильтры и возвращает sliceIterator.
type ErpEZooReader struct {
	products          []ErpProduct
	productBarcodes   []ErpProductBarcode
	categories        []ErpCategory
	locations         []ErpLocation
	suppliers         []ErpSupplier
	supplySpecs       []ErpSupplySpec
	promos            []ErpPromo
	orderRules        []ErpOrderRule
	supplyPlans       []ErpSupplyPlan
	storeAssortments  []ErpStoreAssortment
	storeLifecycle    []ErpStoreAssortmentLifecycleEvent
	masterChangeLog   []ErpMasterChangeLog
	receiptLines      []ErpReceiptLine
	locationStocks    []ErpLocationStockSnapshot
	stockMovements    []ErpStockMovement
	supplierStocks    []ErpSupplierStockSnapshot
}

// New загружает все 16 fixtures-файлов из dir.
// Если supplier_stock_snapshot.json отсутствует или содержит [] — это OK
// (entity_optional). Любая другая ошибка — fatal.
func New(dir string) (*ErpEZooReader, error) {
	r := &ErpEZooReader{}
	type loader struct {
		name string
		dst  any
		opt  bool
	}
	steps := []loader{
		{"products.json", &r.products, false},
		{"product_barcodes.json", &r.productBarcodes, false},
		{"category.json", &r.categories, false},
		{"location.json", &r.locations, false},
		{"supplier.json", &r.suppliers, false},
		{"supply_spec.json", &r.supplySpecs, false},
		{"promo.json", &r.promos, false},
		{"order_rule.json", &r.orderRules, false},
		{"supply_plan.json", &r.supplyPlans, false},
		{"store_assortment.json", &r.storeAssortments, false},
		{"store_assortment_lifecycle_events.json", &r.storeLifecycle, false},
		{"master_change_log.json", &r.masterChangeLog, false},
		{"receipt_line.json", &r.receiptLines, false},
		{"location_stock_snapshot.json", &r.locationStocks, false},
		{"stock_movement.json", &r.stockMovements, false},
		{"supplier_stock_snapshot.json", &r.supplierStocks, true}, // optional
	}
	for _, s := range steps {
		path := filepath.Join(dir, s.name)
		raw, err := os.ReadFile(path) //nolint:gosec // путь приходит из конфига сервиса
		if err != nil {
			if s.opt && os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("erp_e_zoo: read %s: %w", path, err)
		}
		if len(raw) == 0 {
			continue
		}
		if err := json.Unmarshal(raw, s.dst); err != nil {
			return nil, fmt.Errorf("erp_e_zoo: parse %s: %w", path, err)
		}
	}
	return r, nil
}

// Close — no-op для in-memory.
func (r *ErpEZooReader) Close(_ context.Context) error { return nil }

// --- master readers (since-filter применяется по UpdatedAt) ---

func (r *ErpEZooReader) ReadProducts(_ context.Context, since time.Time) (PageIterator[ErpProduct], error) {
	out := make([]ErpProduct, 0, len(r.products))
	for _, p := range r.products {
		if !since.IsZero() && p.UpdatedAt.Before(since) {
			continue
		}
		out = append(out, p)
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadProductBarcodes(_ context.Context, _ time.Time) (PageIterator[ErpProductBarcode], error) {
	return newSliceIterator(r.productBarcodes), nil
}

func (r *ErpEZooReader) ReadCategory(_ context.Context, since time.Time) (PageIterator[ErpCategory], error) {
	out := make([]ErpCategory, 0, len(r.categories))
	for _, c := range r.categories {
		if !since.IsZero() && c.UpdatedAt.Before(since) {
			continue
		}
		out = append(out, c)
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadLocation(_ context.Context, since time.Time) (PageIterator[ErpLocation], error) {
	out := make([]ErpLocation, 0, len(r.locations))
	for _, l := range r.locations {
		if !since.IsZero() && l.UpdatedAt.Before(since) {
			continue
		}
		out = append(out, l)
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadSupplier(_ context.Context, since time.Time) (PageIterator[ErpSupplier], error) {
	out := make([]ErpSupplier, 0, len(r.suppliers))
	for _, s := range r.suppliers {
		if !since.IsZero() && s.UpdatedAt.Before(since) {
			continue
		}
		out = append(out, s)
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadSupplySpec(_ context.Context, _ time.Time) (PageIterator[ErpSupplySpec], error) {
	return newSliceIterator(r.supplySpecs), nil
}

func (r *ErpEZooReader) ReadPromo(_ context.Context, since time.Time) (PageIterator[ErpPromo], error) {
	out := make([]ErpPromo, 0, len(r.promos))
	for _, p := range r.promos {
		if !since.IsZero() && p.UpdatedAt.Before(since) {
			continue
		}
		out = append(out, p)
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadOrderRule(_ context.Context, _ time.Time) (PageIterator[ErpOrderRule], error) {
	return newSliceIterator(r.orderRules), nil
}

func (r *ErpEZooReader) ReadSupplyPlan(_ context.Context, _ time.Time) (PageIterator[ErpSupplyPlan], error) {
	return newSliceIterator(r.supplyPlans), nil
}

func (r *ErpEZooReader) ReadStoreAssortment(_ context.Context, since time.Time) (PageIterator[ErpStoreAssortment], error) {
	out := make([]ErpStoreAssortment, 0, len(r.storeAssortments))
	for _, sa := range r.storeAssortments {
		if !since.IsZero() && sa.UpdatedAt.Before(since) {
			continue
		}
		out = append(out, sa)
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadStoreAssortmentLifecycleEvents(_ context.Context, since time.Time) (PageIterator[ErpStoreAssortmentLifecycleEvent], error) {
	out := make([]ErpStoreAssortmentLifecycleEvent, 0, len(r.storeLifecycle))
	for _, e := range r.storeLifecycle {
		if !since.IsZero() && e.EventDate.Before(since) {
			continue
		}
		out = append(out, e)
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadMasterChangeLog(_ context.Context, since time.Time) (PageIterator[ErpMasterChangeLog], error) {
	out := make([]ErpMasterChangeLog, 0, len(r.masterChangeLog))
	for _, e := range r.masterChangeLog {
		if !since.IsZero() && e.ChangedAt.Before(since) {
			continue
		}
		out = append(out, e)
	}
	return newSliceIterator(out), nil
}

// --- facts readers (event_date range) ---

func inRange(d, from, to time.Time) bool {
	if !from.IsZero() && d.Before(from) {
		return false
	}
	if !to.IsZero() && d.After(to) {
		return false
	}
	return true
}

func (r *ErpEZooReader) ReadReceiptLine(_ context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpReceiptLine], error) {
	out := make([]ErpReceiptLine, 0, len(r.receiptLines))
	for _, e := range r.receiptLines {
		if inRange(e.EventDate, dateFrom, dateTo) {
			out = append(out, e)
		}
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadLocationStockSnapshot(_ context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpLocationStockSnapshot], error) {
	out := make([]ErpLocationStockSnapshot, 0, len(r.locationStocks))
	for _, e := range r.locationStocks {
		if inRange(e.EventDate, dateFrom, dateTo) {
			out = append(out, e)
		}
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadStockMovement(_ context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpStockMovement], error) {
	out := make([]ErpStockMovement, 0, len(r.stockMovements))
	for _, e := range r.stockMovements {
		if inRange(e.EventDate, dateFrom, dateTo) {
			out = append(out, e)
		}
	}
	return newSliceIterator(out), nil
}

func (r *ErpEZooReader) ReadSupplierStockSnapshot(_ context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpSupplierStockSnapshot], error) {
	out := make([]ErpSupplierStockSnapshot, 0, len(r.supplierStocks))
	for _, e := range r.supplierStocks {
		if inRange(e.EventDate, dateFrom, dateTo) {
			out = append(out, e)
		}
	}
	return newSliceIterator(out), nil
}

// compile-time check.
var _ SourceReader = (*ErpEZooReader)(nil)
