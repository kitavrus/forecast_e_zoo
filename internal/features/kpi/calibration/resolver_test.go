package calibration_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/calibration"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

func ptr(s string) *string { return &s }

func mk(kpi, scopeType, scopeID, paramsJSON string) models.KpiCalibration {
	var sid *string
	if scopeID != "" {
		sid = ptr(scopeID)
	}
	return models.KpiCalibration{
		ID:        uuid.New(),
		KpiName:   kpi,
		ScopeType: scopeType,
		ScopeID:   sid,
		Params:    json.RawMessage(paramsJSON),
	}
}

func TestResolver_LocationBeatsGlobal(t *testing.T) {
	t.Parallel()
	all := []models.KpiCalibration{
		mk(constants.KpiOSA, constants.ScopeTypeGlobal, "", `{"x":1}`),
		mk(constants.KpiOSA, constants.ScopeTypeLocation, "loc-1", `{"x":2}`),
	}
	r := calibration.NewResolver(all)
	got := r.Resolve(constants.KpiOSA, calibration.ScopeKeys{LocationID: ptr("loc-1")})
	require.Equal(t, constants.ScopeTypeLocation, got.ScopeType)
	require.JSONEq(t, `{"x":2}`, string(got.Params))
}

func TestResolver_FallsBackToGlobal(t *testing.T) {
	t.Parallel()
	all := []models.KpiCalibration{
		mk(constants.KpiOSA, constants.ScopeTypeGlobal, "", `{"x":1}`),
	}
	r := calibration.NewResolver(all)
	got := r.Resolve(constants.KpiOSA, calibration.ScopeKeys{LocationID: ptr("loc-x")})
	require.Equal(t, constants.ScopeTypeGlobal, got.ScopeType)
	require.JSONEq(t, `{"x":1}`, string(got.Params))
}

func TestResolver_NoCalibrationReturnsEmpty(t *testing.T) {
	t.Parallel()
	r := calibration.NewResolver(nil)
	got := r.Resolve(constants.KpiOSA, calibration.ScopeKeys{})
	require.Equal(t, uuid.Nil, got.ID)
	require.JSONEq(t, `{}`, string(got.Params))
}

func TestResolver_HierarchyOrder_ProductLocationOverLocation(t *testing.T) {
	t.Parallel()
	all := []models.KpiCalibration{
		mk(constants.KpiOSA, constants.ScopeTypeLocation, "loc-1", `{"x":2}`),
		mk(constants.KpiOSA, constants.ScopeTypeProductLocation, "p1|loc-1", `{"x":99}`),
	}
	r := calibration.NewResolver(all)
	got := r.Resolve(constants.KpiOSA, calibration.ScopeKeys{
		ProductLocation: ptr("p1|loc-1"),
		LocationID:      ptr("loc-1"),
	})
	require.JSONEq(t, `{"x":99}`, string(got.Params))
}

func TestResolver_HierarchyOrder_LocationOverSupplierOverCategory(t *testing.T) {
	t.Parallel()
	all := []models.KpiCalibration{
		mk(constants.KpiOTIF, constants.ScopeTypeCategory, "cat-A", `{"v":"cat"}`),
		mk(constants.KpiOTIF, constants.ScopeTypeSupplier, "sup-1", `{"v":"sup"}`),
		mk(constants.KpiOTIF, constants.ScopeTypeLocation, "loc-1", `{"v":"loc"}`),
	}
	r := calibration.NewResolver(all)

	// Все ключи указаны — выигрывает location.
	got := r.Resolve(constants.KpiOTIF, calibration.ScopeKeys{
		LocationID: ptr("loc-1"),
		SupplierID: ptr("sup-1"),
		CategoryID: ptr("cat-A"),
	})
	require.JSONEq(t, `{"v":"loc"}`, string(got.Params))

	// Без location — выигрывает supplier.
	got = r.Resolve(constants.KpiOTIF, calibration.ScopeKeys{
		SupplierID: ptr("sup-1"),
		CategoryID: ptr("cat-A"),
	})
	require.JSONEq(t, `{"v":"sup"}`, string(got.Params))

	// Только category — выигрывает category.
	got = r.Resolve(constants.KpiOTIF, calibration.ScopeKeys{
		CategoryID: ptr("cat-A"),
	})
	require.JSONEq(t, `{"v":"cat"}`, string(got.Params))
}

func TestResolver_DifferentKpiIsolation(t *testing.T) {
	t.Parallel()
	all := []models.KpiCalibration{
		mk(constants.KpiOSA, constants.ScopeTypeGlobal, "", `{"a":1}`),
		mk(constants.KpiOTIF, constants.ScopeTypeGlobal, "", `{"b":2}`),
	}
	r := calibration.NewResolver(all)
	osa := r.Resolve(constants.KpiOSA, calibration.ScopeKeys{})
	otif := r.Resolve(constants.KpiOTIF, calibration.ScopeKeys{})
	require.JSONEq(t, `{"a":1}`, string(osa.Params))
	require.JSONEq(t, `{"b":2}`, string(otif.Params))
}
