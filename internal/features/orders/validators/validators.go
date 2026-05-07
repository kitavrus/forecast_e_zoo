// Package validators — валидация входящих DTO фичи orders.
package validators

import (
	"fmt"

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ValidatePOStatus — query ?status= для GET /v1/orders/purchase-orders.
func ValidatePOStatus(s string) error {
	if !constants.IsKnownPOStatus(s) {
		return errorspkg.ErrInvalidPOStatus
	}
	return nil
}

// ValidateBuildRequest — body POST /build.
//
// Возвращает effective maxPlans.
func ValidateBuildRequest(req *dto.BuildRequest) (int, error) {
	if req == nil || req.MaxPlans == 0 {
		return 0, nil
	}
	if req.MaxPlans < 0 || req.MaxPlans > constants.MaxPlansPerBuildBatch {
		return 0, errorspkg.ErrBadRequest.WithMessage(
			fmt.Sprintf("max_plans must be in [1, %d]", constants.MaxPlansPerBuildBatch))
	}
	return req.MaxPlans, nil
}

// ValidateCancelRequest — body POST /:id/cancel.
func ValidateCancelRequest(req *dto.CancelRequest) error {
	if req == nil {
		return errorspkg.ErrBadRequest.WithMessage("body required")
	}
	if req.Reason == "" {
		return errorspkg.ErrBadRequest.WithMessage("reason is required")
	}
	if len(req.Reason) > constants.MaxReasonLength {
		return errorspkg.ErrBadRequest.WithMessage(
			fmt.Sprintf("reason too long (max %d)", constants.MaxReasonLength))
	}
	if req.ChangedBy == "" {
		return errorspkg.ErrBadRequest.WithMessage("changed_by is required")
	}
	if len(req.ChangedBy) > 100 { //nolint:mnd
		return errorspkg.ErrBadRequest.WithMessage("changed_by too long (max 100)")
	}
	return nil
}

// ValidateRegenerateRequest — body POST /:id/regenerate.
func ValidateRegenerateRequest(req *dto.RegenerateRequest) error {
	if req == nil {
		return errorspkg.ErrBadRequest.WithMessage("body required")
	}
	if len(req.Reason) > constants.MaxReasonLength {
		return errorspkg.ErrBadRequest.WithMessage(
			fmt.Sprintf("reason too long (max %d)", constants.MaxReasonLength))
	}
	if req.ChangedBy == "" {
		return errorspkg.ErrBadRequest.WithMessage("changed_by is required")
	}
	if len(req.ChangedBy) > 100 { //nolint:mnd
		return errorspkg.ErrBadRequest.WithMessage("changed_by too long (max 100)")
	}
	return nil
}
