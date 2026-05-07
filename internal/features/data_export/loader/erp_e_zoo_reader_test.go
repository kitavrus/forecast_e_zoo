package loader_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/loader"
)

// fixturesDir возвращает абсолютный путь до testdata/fixtures.
func fixturesDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// here = .../internal/features/data_export/loader/erp_e_zoo_reader_test.go
	root := filepath.Join(filepath.Dir(here), "..", "..", "..", "..")
	return filepath.Join(root, "testdata", "fixtures")
}

func newReader(t *testing.T) *loader.ErpEZooReader {
	t.Helper()
	r, err := loader.New(fixturesDir(t))
	require.NoError(t, err)
	return r
}

func collect[T any](t *testing.T, it loader.PageIterator[T]) []T {
	t.Helper()
	defer func() { _ = it.Close() }()
	out := make([]T, 0)
	for it.Next(context.Background()) {
		out = append(out, it.Item())
	}
	require.NoError(t, it.Err())
	return out
}

func TestErpEZooReader_ReadProducts_HappyPath(t *testing.T) {
	t.Parallel()
	r := newReader(t)
	it, err := r.ReadProducts(context.Background(), time.Time{})
	require.NoError(t, err)
	prods := collect(t, it)
	require.GreaterOrEqual(t, len(prods), 5)
	require.Equal(t, "P-001", prods[0].ID)
}

func TestErpEZooReader_ReadProducts_SinceFilter(t *testing.T) {
	t.Parallel()
	r := newReader(t)
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	it, err := r.ReadProducts(context.Background(), since)
	require.NoError(t, err)
	prods := collect(t, it)
	for _, p := range prods {
		require.False(t, p.UpdatedAt.Before(since), "product %s leaked through since filter (%v)", p.ID, p.UpdatedAt)
	}
	require.GreaterOrEqual(t, len(prods), 1)
}

func TestErpEZooReader_ReadReceiptLine_DateRangeFilter(t *testing.T) {
	t.Parallel()
	r := newReader(t)
	from := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	it, err := r.ReadReceiptLine(context.Background(), from, to)
	require.NoError(t, err)
	lines := collect(t, it)
	for _, l := range lines {
		require.False(t, l.EventDate.Before(from))
		require.False(t, l.EventDate.After(to))
	}
}

func TestErpEZooReader_SupplierStock_EmptyOK(t *testing.T) {
	t.Parallel()
	r := newReader(t)
	it, err := r.ReadSupplierStockSnapshot(context.Background(), time.Time{}, time.Time{})
	require.NoError(t, err)
	rows := collect(t, it)
	require.Empty(t, rows)
}

func TestPageIterator_NextThenItem(t *testing.T) {
	t.Parallel()
	r := newReader(t)
	it, err := r.ReadCategory(context.Background(), time.Time{})
	require.NoError(t, err)
	require.True(t, it.Next(context.Background()))
	first := it.Item()
	require.NotEmpty(t, first.ID)
	_ = it.Close()
}

func TestPageIterator_AfterCloseReturnsFalse(t *testing.T) {
	t.Parallel()
	r := newReader(t)
	it, err := r.ReadLocation(context.Background(), time.Time{})
	require.NoError(t, err)
	require.NoError(t, it.Close())
	require.False(t, it.Next(context.Background()))
}

func TestErpEZooReader_AllSourcesCallable(t *testing.T) {
	t.Parallel()
	r := newReader(t)
	ctx := context.Background()
	since := time.Time{}

	// master
	for name, fn := range map[string]func() error{
		"products":         func() error { _, e := r.ReadProducts(ctx, since); return e },
		"product_barcodes": func() error { _, e := r.ReadProductBarcodes(ctx, since); return e },
		"category":         func() error { _, e := r.ReadCategory(ctx, since); return e },
		"location":         func() error { _, e := r.ReadLocation(ctx, since); return e },
		"supplier":         func() error { _, e := r.ReadSupplier(ctx, since); return e },
		"supply_spec":      func() error { _, e := r.ReadSupplySpec(ctx, since); return e },
		"promo":            func() error { _, e := r.ReadPromo(ctx, since); return e },
		"order_rule":       func() error { _, e := r.ReadOrderRule(ctx, since); return e },
		"supply_plan":      func() error { _, e := r.ReadSupplyPlan(ctx, since); return e },
		"store_assortment": func() error { _, e := r.ReadStoreAssortment(ctx, since); return e },
		"lifecycle":        func() error { _, e := r.ReadStoreAssortmentLifecycleEvents(ctx, since); return e },
		"master_change":    func() error { _, e := r.ReadMasterChangeLog(ctx, since); return e },
	} {
		require.NoErrorf(t, fn(), "reader %s", name)
	}
	// facts
	for name, fn := range map[string]func() error{
		"receipt_line":            func() error { _, e := r.ReadReceiptLine(ctx, time.Time{}, time.Time{}); return e },
		"location_stock_snapshot": func() error { _, e := r.ReadLocationStockSnapshot(ctx, time.Time{}, time.Time{}); return e },
		"stock_movement":          func() error { _, e := r.ReadStockMovement(ctx, time.Time{}, time.Time{}); return e },
	} {
		require.NoErrorf(t, fn(), "fact reader %s", name)
	}

	require.NoError(t, r.Close(ctx))
}
