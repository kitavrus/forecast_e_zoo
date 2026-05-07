package loader_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/loader"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/validation"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

func newEngine() *validation.Engine {
	rules := []validation.Rule{
		{
			ID: "products.duplicate_pk", Entity: "products",
			Check: "duplicate_pk", Field: "id", Severity: validation.SeverityCritical,
		},
	}
	return validation.New(rules, nil)
}

func TestLoader_HappyPath(t *testing.T) {
	t.Parallel()
	r := &mockReader{products: []loader.ErpProduct{
		{ID: "P-1", SKU: "S1", Name: "n", Unit: "kg", IsActive: true},
		{ID: "P-2", SKU: "S2", Name: "n", Unit: "kg", IsActive: true},
	}}
	repo := &mockRepo{}
	l := loader.NewLoader(r, repo, newEngine(), loader.Options{})
	id, err := l.Run(context.Background(), "test")
	require.NoError(t, err)
	require.NotEqual(t, "", id.String())

	require.Equal(t, 1, repo.insertRunningCalled)
	require.GreaterOrEqual(t, repo.upsertProductCalled, 2)
	require.Equal(t, 1, repo.flipCalled)
	require.Equal(t, 1, repo.markCommittedCalled)
	require.Equal(t, 0, repo.markFailedCalled)
}

func TestLoader_QualityThresholdExceeded(t *testing.T) {
	t.Parallel()
	// 10 продуктов, 2 с дубликатами PK → 20% failed (порог 1%).
	r := &mockReader{products: []loader.ErpProduct{
		{ID: "P-1", SKU: "S", Name: "n", Unit: "kg"}, {ID: "P-2", SKU: "S", Name: "n", Unit: "kg"},
		{ID: "P-3", SKU: "S", Name: "n", Unit: "kg"}, {ID: "P-4", SKU: "S", Name: "n", Unit: "kg"},
		{ID: "P-5", SKU: "S", Name: "n", Unit: "kg"}, {ID: "P-6", SKU: "S", Name: "n", Unit: "kg"},
		{ID: "P-7", SKU: "S", Name: "n", Unit: "kg"}, {ID: "P-8", SKU: "S", Name: "n", Unit: "kg"},
		{ID: "P-1", SKU: "S", Name: "n", Unit: "kg"}, // duplicate
		{ID: "P-2", SKU: "S", Name: "n", Unit: "kg"}, // duplicate
	}}
	repo := &mockRepo{}
	l := loader.NewLoader(r, repo, newEngine(), loader.Options{})
	_, err := l.Run(context.Background(), "test")
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrQualityThresholdExceeded))
	require.Equal(t, 0, repo.flipCalled)
	require.Equal(t, 1, repo.markFailedCalled)
}

func TestLoader_ReaderError_MarksLoadFailed(t *testing.T) {
	t.Parallel()
	r := &mockReader{productsErr: errors.New("erp 503")}
	repo := &mockRepo{}
	l := loader.NewLoader(r, repo, newEngine(), loader.Options{})
	_, err := l.Run(context.Background(), "test")
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrERPUnavailable))
	require.Equal(t, 1, repo.markFailedCalled)
	require.Equal(t, 0, repo.flipCalled)
}

func TestLoader_OptionalEntity_SupplierStockEmpty_OK(t *testing.T) {
	t.Parallel()
	r := &mockReader{} // supplier_stock пуст
	repo := &mockRepo{}
	l := loader.NewLoader(r, repo, newEngine(), loader.Options{})
	_, err := l.Run(context.Background(), "test")
	require.NoError(t, err)
	require.Equal(t, 1, repo.flipCalled)
}

func TestLoader_DuplicatePK_AddsToRejectLog(t *testing.T) {
	t.Parallel()
	// 1 хороший + 1 дубль (50% failed → quality threshold не пройдёт, но мы проверяем
	// именно то, что reject_log получил вызов).
	r := &mockReader{products: []loader.ErpProduct{
		{ID: "P-1", SKU: "S", Name: "n", Unit: "kg"},
		{ID: "P-1", SKU: "S", Name: "n", Unit: "kg"}, // duplicate
	}}
	repo := &mockRepo{}
	l := loader.NewLoader(r, repo, newEngine(), loader.Options{})
	_, _ = l.Run(context.Background(), "test")
	require.GreaterOrEqual(t, repo.insertRejectCalled, 1)
}

func TestLoader_FlipFailure_MarksFailed(t *testing.T) {
	t.Parallel()
	r := &mockReader{products: []loader.ErpProduct{{ID: "P-1", SKU: "S", Name: "n", Unit: "kg"}}}
	repo := &mockRepo{flipErr: errors.New("snapshot busy")}
	l := loader.NewLoader(r, repo, newEngine(), loader.Options{})
	_, err := l.Run(context.Background(), "test")
	require.Error(t, err)
	require.Equal(t, 1, repo.markFailedCalled)
}

func TestLoader_EntityOrder_MasterBeforeFacts(t *testing.T) {
	t.Parallel()
	require.Less(t, findIndex(loader.EntityOrder, "products"), findIndex(loader.EntityOrder, "receipt_line"))
	require.Less(t, findIndex(loader.EntityOrder, "category"), findIndex(loader.EntityOrder, "receipt_line"))
	require.Less(t, findIndex(loader.EntityOrder, "location"), findIndex(loader.EntityOrder, "stock_movement"))
}

func findIndex(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}

// _ helper.
var _ = time.Now
