package builder_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/orders/builder"
	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
)

func ptr[T any](v T) *T { return &v }

func TestBuild_HappyPath_AllPricesFromProduct(t *testing.T) {
	t.Parallel()
	planID := uuid.New()
	runID := uuid.New()
	created := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	out := builder.Build(builder.Inputs{
		Plan: models.ApprovedPlan{
			ID:         planID,
			RunID:      runID,
			SupplierID: "SUP-1",
			LocationID: "LOC-1",
		},
		Lines: []models.PlanLine{
			{ProductID: "P1", ReorderQty: 10},
			{ProductID: "P2", ReorderQty: 5},
		},
		Supplier: models.SupplierMaster{
			SupplierID:   "SUP-1",
			Currency:     "EUR",
			LeadTimeDays: 3,
			HasMartRow:   true,
		},
		Products: map[string]models.ProductMaster{
			"P1": {ProductID: "P1", UnitPrice: ptr(2.0), HasMartRow: true},
			"P2": {ProductID: "P2", UnitPrice: ptr(4.0), HasMartRow: true},
		},
		PONumber:  "PO-20260507-000001",
		CreatedAt: created,
	})

	assert.Equal(t, "PO-20260507-000001", out.Order.PONumber)
	assert.Equal(t, "EUR", out.Order.Currency)
	assert.NotNil(t, out.Order.DeliveryDate)
	assert.Equal(t, "2026-05-10", *out.Order.DeliveryDate) // +3
	require.NotNil(t, out.Order.TotalAmount)
	assert.InDelta(t, 40.0, *out.Order.TotalAmount, 0.0001) // 10*2 + 5*4
	assert.InDelta(t, 15.0, out.Order.TotalQty, 0.0001)
	assert.Len(t, out.Lines, 2)
	for _, l := range out.Lines {
		assert.Equal(t, constants.PricingSourceProduct, l.PricingSource)
		assert.NotNil(t, l.UnitPrice)
		assert.NotNil(t, l.LineAmount)
	}
}

func TestBuild_FallbackToSupplierDefault(t *testing.T) {
	t.Parallel()
	out := builder.Build(builder.Inputs{
		Plan: models.ApprovedPlan{ID: uuid.New(), SupplierID: "S", LocationID: "L"},
		Lines: []models.PlanLine{
			{ProductID: "P1", ReorderQty: 10},
		},
		Supplier: models.SupplierMaster{
			SupplierID:       "S",
			Currency:         "UAH",
			DefaultUnitPrice: ptr(1.5),
			HasMartRow:       true,
		},
		Products:  map[string]models.ProductMaster{},
		CreatedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	})
	require.Len(t, out.Lines, 1)
	assert.Equal(t, constants.PricingSourceSupplierDefault, out.Lines[0].PricingSource)
	require.NotNil(t, out.Order.TotalAmount)
	assert.InDelta(t, 15.0, *out.Order.TotalAmount, 0.0001)
}

func TestBuild_MissingPriceLeavesTotalNull(t *testing.T) {
	t.Parallel()
	out := builder.Build(builder.Inputs{
		Plan: models.ApprovedPlan{ID: uuid.New(), SupplierID: "S", LocationID: "L"},
		Lines: []models.PlanLine{
			{ProductID: "P1", ReorderQty: 5},
		},
		Supplier: models.SupplierMaster{SupplierID: "S", Currency: "UAH", HasMartRow: true},
		// No products map, no supplier default
		CreatedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	})
	require.Len(t, out.Lines, 1)
	assert.Equal(t, constants.PricingSourceMissing, out.Lines[0].PricingSource)
	assert.Nil(t, out.Lines[0].UnitPrice)
	assert.Nil(t, out.Lines[0].LineAmount)
	assert.Nil(t, out.Order.TotalAmount)
	require.NotNil(t, out.Order.Notes)
	assert.Contains(t, *out.Order.Notes, "missing unit_price")
}

func TestBuild_NoSupplierMartRow_DefaultsAndWarning(t *testing.T) {
	t.Parallel()
	out := builder.Build(builder.Inputs{
		Plan: models.ApprovedPlan{ID: uuid.New(), SupplierID: "S-UNKNOWN", LocationID: "L"},
		Lines: []models.PlanLine{
			{ProductID: "P1", ReorderQty: 1},
		},
		Supplier: models.SupplierMaster{
			SupplierID: "S-UNKNOWN",
			HasMartRow: false, // no marts row
		},
		Products: map[string]models.ProductMaster{
			"P1": {ProductID: "P1", UnitPrice: ptr(10.0), HasMartRow: true},
		},
		CreatedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	})
	assert.Equal(t, constants.DefaultCurrency, out.Order.Currency)
	require.NotNil(t, out.Order.DeliveryDate)
	// default lead time = 7 days → 2026-05-14
	assert.Equal(t, "2026-05-14", *out.Order.DeliveryDate)
	require.NotNil(t, out.Order.Notes)
	assert.Contains(t, *out.Order.Notes, "no master row")
}

func TestBuild_SkipsZeroAndNegativeQty(t *testing.T) {
	t.Parallel()
	out := builder.Build(builder.Inputs{
		Plan: models.ApprovedPlan{ID: uuid.New(), SupplierID: "S", LocationID: "L"},
		Lines: []models.PlanLine{
			{ProductID: "P1", ReorderQty: 0}, // skipped
			{ProductID: "P2", ReorderQty: -1}, // skipped
			{ProductID: "P3", ReorderQty: 5},
		},
		Supplier: models.SupplierMaster{SupplierID: "S", Currency: "UAH", HasMartRow: true},
		Products: map[string]models.ProductMaster{
			"P3": {ProductID: "P3", UnitPrice: ptr(2.0), HasMartRow: true},
		},
		CreatedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	})
	assert.Len(t, out.Lines, 1)
	assert.Equal(t, "P3", out.Lines[0].ProductID)
	assert.InDelta(t, 5.0, out.Order.TotalQty, 0.0001)
}

func TestBuild_EmptyLines(t *testing.T) {
	t.Parallel()
	out := builder.Build(builder.Inputs{
		Plan:      models.ApprovedPlan{ID: uuid.New(), SupplierID: "S", LocationID: "L"},
		Lines:     []models.PlanLine{},
		Supplier:  models.SupplierMaster{SupplierID: "S", Currency: "UAH", HasMartRow: true},
		Products:  map[string]models.ProductMaster{},
		CreatedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	})
	assert.Empty(t, out.Lines)
	assert.InDelta(t, 0.0, out.Order.TotalQty, 0.0001)
	assert.Nil(t, out.Order.TotalAmount)
}
