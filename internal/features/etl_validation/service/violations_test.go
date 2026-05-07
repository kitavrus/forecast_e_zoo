package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
)

func TestViolationsToRejectEntries_AllFields(t *testing.T) {
	t.Parallel()
	runID := uuid.New()
	vs := []validation.Violation{
		{
			RuleName:    "products_unique_sku",
			Kind:        "unique_business_key",
			Entity:      "product",
			Field:       "sku",
			BusinessKey: "A",
			Severity:    validation.SeverityCritical,
			Message:     "duplicate sku",
		},
		{
			RuleName: "stock_fk_product",
			Kind:     "fk_exists",
			Entity:   "stock_on_hand",
			Severity: validation.SeveritySoft,
			Message:  "missing parent",
		},
	}
	got := violationsToRejectEntries(runID, vs)
	require.Len(t, got, 2)

	assert.Equal(t, runID, got[0].EtlRunID)
	assert.Equal(t, "product", got[0].Entity)
	assert.Equal(t, "products_unique_sku", got[0].RuleID)
	assert.Equal(t, "critical", got[0].Severity)
	require.NotNil(t, got[0].BusinessKey)
	assert.Equal(t, "A", *got[0].BusinessKey)
	require.NotNil(t, got[0].Field)
	assert.Equal(t, "sku", *got[0].Field)

	// Second violation has no field/business_key — pointers remain nil.
	assert.Equal(t, runID, got[1].EtlRunID)
	assert.Nil(t, got[1].BusinessKey)
	assert.Nil(t, got[1].Field)
	assert.Equal(t, "soft", got[1].Severity)
}

func TestViolationsToRejectEntries_Empty(t *testing.T) {
	t.Parallel()
	got := violationsToRejectEntries(uuid.New(), nil)
	assert.Empty(t, got)
}
