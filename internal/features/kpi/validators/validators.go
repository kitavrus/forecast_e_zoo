// Package validators — валидация входящих DTO фичи kpi.
package validators

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// dateLayout — ISO date YYYY-MM-DD.
const dateLayout = "2006-01-02"

// ValidateKpiName — проверка значения kpi_name.
func ValidateKpiName(name string) error {
	if !constants.IsKnownKpi(name) {
		return errorspkg.ErrInvalidKpiName.WithMessage(fmt.Sprintf("unknown kpi: %q", name))
	}
	return nil
}

// ValidateScopeType — проверка значения scope_type.
func ValidateScopeType(scope string) error {
	if !constants.IsKnownScopeType(scope) {
		return errorspkg.ErrBadRequest.WithMessage(fmt.Sprintf("unknown scope_type: %q", scope))
	}
	return nil
}

// ValidateAsOfDate — проверка query-param ?as_of_date=YYYY-MM-DD.
func ValidateAsOfDate(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, errorspkg.ErrBadRequest.WithMessage(fmt.Sprintf("invalid as_of_date: %s", s))
	}
	return t, nil
}

// ValidateUpdateCalibration — body PUT /v1/kpi/calibrations/:id.
func ValidateUpdateCalibration(req *dto.UpdateCalibrationRequest) error {
	if req == nil || len(req.Params) == 0 {
		return errorspkg.ErrBadRequest.WithMessage("params is required")
	}
	var v map[string]interface{}
	if err := json.Unmarshal(req.Params, &v); err != nil {
		return errorspkg.ErrBadRequest.WithMessage("params must be a JSON object")
	}
	if len(v) == 0 {
		return errorspkg.ErrBadRequest.WithMessage("params object cannot be empty")
	}
	return nil
}

// ValidateRefreshRequest — body POST /v1/kpi/snapshots/refresh.
//
// Возвращает (effectiveDate, kpiNames, error).
func ValidateRefreshRequest(req *dto.RefreshSnapshotsRequest) (time.Time, []string, error) {
	asOf := time.Now().UTC().Truncate(24 * time.Hour) //nolint:mnd // 1 day truncation
	if req != nil && req.FromDate != nil && *req.FromDate != "" {
		t, err := time.Parse(dateLayout, *req.FromDate)
		if err != nil {
			return time.Time{}, nil, errorspkg.ErrBadRequest.WithMessage(fmt.Sprintf("invalid from_date: %s", *req.FromDate))
		}
		asOf = t
	}
	var kpis []string
	if req != nil && len(req.KpiNames) > 0 {
		for _, k := range req.KpiNames {
			if err := ValidateKpiName(k); err != nil {
				return time.Time{}, nil, err
			}
		}
		kpis = req.KpiNames
	}
	return asOf, kpis, nil
}
