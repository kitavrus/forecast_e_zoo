package validation_test

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/validation"
)

// projectConfigPath возвращает абсолютный путь до configs/validation_rules.yaml.
func projectConfigPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// engine_test.go → внутри internal/features/data_export/validation/
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return filepath.Join(root, "configs", "validation_rules.yaml")
}

func TestEngine_LoadYAML(t *testing.T) {
	t.Parallel()
	eng, err := validation.Load(projectConfigPath(t))
	require.NoError(t, err)
	require.NotNil(t, eng)
	require.True(t, eng.IsEntityOptional("supplier_stock_snapshot"))
	require.GreaterOrEqual(t, len(eng.Rules()), 7)
}

func TestEngine_NegativeQty_Critical(t *testing.T) {
	t.Parallel()
	eng := validation.New([]validation.Rule{{
		ID: "r1", Entity: "receipt_line", Check: "negative_qty",
		Field: "qty", Severity: validation.SeverityCritical,
	}}, nil)
	state := validation.NewState("L1")
	v := eng.Check("receipt_line", map[string]any{"qty": -3.0}, state)
	require.Len(t, v, 1)
	require.Equal(t, validation.SeverityCritical, v[0].Severity)
}

func TestEngine_FutureEventTime_Critical(t *testing.T) {
	t.Parallel()
	eng := validation.New([]validation.Rule{{
		ID: "r2", Entity: "receipt_line", Check: "future_event_time",
		Field: "event_time", Severity: validation.SeverityCritical,
	}}, nil)
	state := validation.NewState("L1")
	future := time.Now().Add(2 * time.Hour)
	v := eng.Check("receipt_line", map[string]any{"event_time": future}, state)
	require.Len(t, v, 1)
	require.Equal(t, validation.SeverityCritical, v[0].Severity)
}

func TestEngine_DuplicatePK_Critical(t *testing.T) {
	t.Parallel()
	eng := validation.New([]validation.Rule{{
		ID: "r3", Entity: "products", Check: "duplicate_pk",
		Field: "id", Severity: validation.SeverityCritical,
	}}, nil)
	state := validation.NewState("L1")
	first := eng.Check("products", map[string]any{"id": "P-1"}, state)
	require.Empty(t, first)
	second := eng.Check("products", map[string]any{"id": "P-1"}, state)
	require.Len(t, second, 1)
	require.Equal(t, validation.SeverityCritical, second[0].Severity)
}

func TestEngine_MissingField_Soft(t *testing.T) {
	t.Parallel()
	eng := validation.New([]validation.Rule{{
		ID: "r4", Entity: "location", Check: "missing_required",
		Fields: []string{"id", "name", "type"}, Severity: validation.SeveritySoft,
	}}, nil)
	state := validation.NewState("L1")
	v := eng.Check("location", map[string]any{"id": "L1"}, state) // missing name, type
	require.Len(t, v, 1)
	require.Equal(t, validation.SeveritySoft, v[0].Severity)
	require.Contains(t, v[0].Message, "name")
}

func TestEngine_NumericRange_Soft(t *testing.T) {
	t.Parallel()
	maxVal := 90.0
	minVal := 0.0
	eng := validation.New([]validation.Rule{{
		ID: "r5", Entity: "supply_spec", Check: "numeric_range",
		Field: "lead_time_days", Min: &minVal, Max: &maxVal,
		Severity: validation.SeveritySoft,
	}}, nil)
	state := validation.NewState("L1")
	v := eng.Check("supply_spec", map[string]any{"lead_time_days": 365}, state)
	require.Len(t, v, 1)
	require.Equal(t, validation.SeveritySoft, v[0].Severity)
}

func TestEngine_RegexMatch(t *testing.T) {
	t.Parallel()
	eng := validation.New([]validation.Rule{{
		ID: "r6", Entity: "product_barcodes", Check: "regex_match",
		Field: "barcode", Pattern: `^[0-9]{8,14}$`,
		Severity: validation.SeveritySoft,
	}}, nil)
	state := validation.NewState("L1")
	bad := eng.Check("product_barcodes", map[string]any{"barcode": "BAD-123"}, state)
	require.Len(t, bad, 1)
	good := eng.Check("product_barcodes", map[string]any{"barcode": "1234567890123"}, state)
	require.Empty(t, good)
}

func TestEngine_NoViolations(t *testing.T) {
	t.Parallel()
	eng := validation.New([]validation.Rule{{
		ID: "r7", Entity: "receipt_line", Check: "negative_qty",
		Field: "qty", Severity: validation.SeverityCritical,
	}}, nil)
	state := validation.NewState("L1")
	v := eng.Check("receipt_line", map[string]any{"qty": 5.0}, state)
	require.Empty(t, v)
}

func TestEngine_OptionalEntity_NoViolation(t *testing.T) {
	t.Parallel()
	eng := validation.New(nil, []string{"supplier_stock_snapshot"})
	state := validation.NewState("L1")
	v := eng.Check("supplier_stock_snapshot", map[string]any{"qty_available": 0}, state)
	require.Empty(t, v)
	require.True(t, eng.IsEntityOptional("supplier_stock_snapshot"))
}
