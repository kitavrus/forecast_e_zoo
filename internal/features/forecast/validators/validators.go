// Package validators — валидация входящих DTO фичи forecast.
package validators

import (
	"fmt"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/constants"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// dateLayout — ISO date YYYY-MM-DD.
const dateLayout = "2006-01-02"

// ValidateRunStatus — query ?status= для GET /v1/forecast/runs.
func ValidateRunStatus(s string) error {
	if !constants.IsKnownRunStatus(s) {
		return errorspkg.ErrBadRequest.WithMessage(fmt.Sprintf("invalid run status: %q", s))
	}
	return nil
}

// ValidatePlanStatus — query ?status= для GET /v1/replenishment/plans.
func ValidatePlanStatus(s string) error {
	if !constants.IsKnownPlanStatus(s) {
		return errorspkg.ErrInvalidPlanStatus
	}
	return nil
}

// ParseDate — YYYY-MM-DD.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, errorspkg.ErrBadRequest.WithMessage(fmt.Sprintf("invalid date: %s", s))
	}
	return t, nil
}

// ParseTimestamp — RFC3339 (для ?from=&to=).
func ParseTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, errorspkg.ErrBadRequest.WithMessage(fmt.Sprintf("invalid timestamp: %s", s))
	}
	return t, nil
}

// ValidateRefreshRequest — body POST /v1/forecast/runs/refresh.
//
// Возвращает effective horizonDays.
func ValidateRefreshRequest(req *dto.RefreshRunRequest) (int, error) {
	horizon := constants.HorizonDefault
	if req != nil && req.HorizonDays != nil {
		h := *req.HorizonDays
		if h < constants.HorizonMin || h > constants.HorizonMax {
			return 0, errorspkg.ErrInvalidHorizon
		}
		horizon = h
	}
	return horizon, nil
}

// ValidateApproveRequest — body POST /v1/replenishment/plans/:id/approve.
func ValidateApproveRequest(req *dto.ApprovePlanRequest) error {
	if req == nil || req.ApprovedBy == "" {
		return errorspkg.ErrBadRequest.WithMessage("approved_by is required")
	}
	if len(req.ApprovedBy) > 100 { //nolint:mnd // sane upper bound
		return errorspkg.ErrBadRequest.WithMessage("approved_by too long (max 100)")
	}
	return nil
}
