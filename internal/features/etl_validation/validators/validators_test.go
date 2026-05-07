package validators_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

func TestValidatePostEtlRun(t *testing.T) {
	t.Parallel()
	v := validators.New()
	require.NoError(t, v.ValidatePostEtlRun(validators.PostEtlRunInput{}))
}

func TestValidateRetryEtlRun(t *testing.T) {
	t.Parallel()
	v := validators.New()
	require.NoError(t, v.ValidateRetryEtlRun(uuid.New().String()))

	err := v.ValidateRetryEtlRun("not-a-uuid")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errorspkg.ErrBadRequest), "expected ErrBadRequest, got %v", err)

	err = v.ValidateRetryEtlRun("")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errorspkg.ErrBadRequest))
}

func TestValidateGetEtlRun(t *testing.T) {
	t.Parallel()
	v := validators.New()
	require.NoError(t, v.ValidateGetEtlRun(uuid.New().String()))
	require.Error(t, v.ValidateGetEtlRun("bad"))
}

func TestValidateListEtlRuns(t *testing.T) {
	t.Parallel()
	v := validators.New()
	tests := []struct {
		name    string
		query   validators.ListEtlRunsQuery
		wantErr bool
	}{
		{"empty all", validators.ListEtlRunsQuery{}, false},
		{"valid status", validators.ListEtlRunsQuery{Status: "running"}, false},
		{"bad status", validators.ListEtlRunsQuery{Status: "unknown"}, true},
		{"valid kind", validators.ListEtlRunsQuery{Kind: "full"}, false},
		{"bad kind", validators.ListEtlRunsQuery{Kind: "weird"}, true},
		{"valid cursor", validators.ListEtlRunsQuery{Cursor: "2026-01-01T00:00:00Z"}, false},
		{"bad cursor", validators.ListEtlRunsQuery{Cursor: "yesterday"}, true},
		{"limit ok", validators.ListEtlRunsQuery{Limit: 50}, false},
		{"limit too low", validators.ListEtlRunsQuery{Limit: -1}, true},
		{"limit too high", validators.ListEtlRunsQuery{Limit: 1000}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateListEtlRuns(tc.query)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, errorspkg.ErrBadRequest))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMartRefresh(t *testing.T) {
	t.Parallel()
	v := validators.New()
	tests := []struct {
		name    string
		input   string
		wantErr error // sentinel reference
	}{
		{"empty", "", errorspkg.ErrBadRequest},
		{"unknown mart", "mart_x", errorspkg.ErrBadRequest},
		{"known but not refreshable", "mart_demand_history", errorspkg.ErrMartRefreshNotSupported},
		{"refreshable", "mart_supplier_scorecard", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateMartRefresh(tc.input)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateListRejectLog(t *testing.T) {
	t.Parallel()
	v := validators.New()
	validRunID := uuid.New().String()
	tests := []struct {
		name    string
		query   validators.ListRejectLogQuery
		wantErr bool
	}{
		{"empty", validators.ListRejectLogQuery{}, false},
		{"valid run id", validators.ListRejectLogQuery{EtlRunID: validRunID}, false},
		{"bad run id", validators.ListRejectLogQuery{EtlRunID: "abc"}, true},
		{"valid severity", validators.ListRejectLogQuery{Severity: "critical"}, false},
		{"bad severity", validators.ListRejectLogQuery{Severity: "warning"}, true},
		{"valid entity", validators.ListRejectLogQuery{Entity: "receipt_line"}, false},
		{"bad entity", validators.ListRejectLogQuery{Entity: "asteroid"}, true},
		{"valid cursor", validators.ListRejectLogQuery{Cursor: "12345"}, false},
		{"bad cursor — non-numeric", validators.ListRejectLogQuery{Cursor: "abc"}, true},
		{"bad cursor — zero", validators.ListRejectLogQuery{Cursor: "0"}, true},
		{"limit ok", validators.ListRejectLogQuery{Limit: 100}, false},
		{"limit too high", validators.ListRejectLogQuery{Limit: 9999}, true},
		{"limit negative", validators.ListRejectLogQuery{Limit: -10}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateListRejectLog(tc.query)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, errorspkg.ErrBadRequest))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidatorInterface — assert *Impl реализует Validator.
func TestValidatorInterface(t *testing.T) {
	t.Parallel()
	var _ validators.Validator = validators.New()
}
