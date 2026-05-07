// Package validators — валидация входящих DTO фичи channels.
package validators

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ValidateChannelType — query/body channel_type.
func ValidateChannelType(s string) error {
	if !constants.IsKnownChannelType(s) {
		return errorspkg.ErrInvalidChannelType
	}
	return nil
}

// ValidateAuthMode — query/body auth_mode.
func ValidateAuthMode(s string) error {
	if !constants.IsKnownAuthMode(s) {
		return errorspkg.ErrInvalidAuthMode
	}
	return nil
}

// ValidateSendAttemptStatus — query ?status= для list.
func ValidateSendAttemptStatus(s string) error {
	if !constants.IsKnownSendAttemptStatus(s) {
		return errorspkg.ErrBadRequest.WithMessage(
			"invalid status (allowed: pending|success|failed|skipped)")
	}
	return nil
}

// ValidateListFilter — собирает models.SendAttemptFilter из query.
//
//nolint:gocognit,cyclop // прямолинейный набор условий, разбиение бесполезно
func ValidateListFilter(
	poIDStr, supplierID, status, fromStr, toStr, cursor string, limit int,
) (models.SendAttemptFilter, error) {
	f := models.SendAttemptFilter{
		Cursor: cursor,
	}
	if limit <= 0 {
		f.Limit = constants.LimitDefault
	} else if limit > constants.LimitMax {
		return f, errorspkg.ErrBadRequest.WithMessage(
			fmt.Sprintf("limit must be <= %d", constants.LimitMax))
	} else {
		f.Limit = limit
	}
	if poIDStr != "" {
		id, err := uuid.Parse(poIDStr)
		if err != nil {
			return f, errorspkg.ErrBadRequest.WithMessage("invalid po_id")
		}
		f.POID = &id
	}
	if supplierID != "" {
		v := strings.TrimSpace(supplierID)
		f.SupplierID = &v
	}
	if status != "" {
		if err := ValidateSendAttemptStatus(status); err != nil {
			return f, err
		}
		v := status
		f.Status = &v
	}
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return f, errorspkg.ErrBadRequest.WithMessage("invalid from (RFC3339 expected)")
		}
		f.From = &t
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return f, errorspkg.ErrBadRequest.WithMessage("invalid to (RFC3339 expected)")
		}
		f.To = &t
	}
	return f, nil
}

// ValidateUpsertConfig — body PUT /v1/channels/configs/:supplier_id.
//
//nolint:cyclop // последовательные проверки полей
func ValidateUpsertConfig(
	supplierID string, req *dto.UpsertChannelConfigRequest,
) (models.UpsertChannelConfigInput, error) {
	out := models.UpsertChannelConfigInput{}
	if req == nil {
		return out, errorspkg.ErrBadRequest.WithMessage("body required")
	}
	supplierID = strings.TrimSpace(supplierID)
	if supplierID == "" {
		return out, errorspkg.ErrBadRequest.WithMessage("supplier_id is required")
	}
	if err := ValidateChannelType(req.ChannelType); err != nil {
		return out, err
	}
	if err := ValidateAuthMode(req.AuthMode); err != nil {
		return out, err
	}
	if strings.TrimSpace(req.EndpointURL) == "" {
		return out, errorspkg.ErrBadRequest.WithMessage("endpoint_url is required")
	}
	timeout := req.TimeoutSec
	if timeout <= 0 {
		timeout = constants.DefaultTimeoutSec
	}
	retry := req.RetryMax
	if retry < 0 {
		return out, errorspkg.ErrBadRequest.WithMessage("retry_max must be >= 0")
	}
	if retry == 0 {
		retry = constants.DefaultRetryMax
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	out = models.UpsertChannelConfigInput{
		SupplierID:         supplierID,
		ChannelType:        req.ChannelType,
		EndpointURL:        strings.TrimSpace(req.EndpointURL),
		AuthMode:           req.AuthMode,
		AuthCredentialsRef: req.AuthCredentialsRef,
		TimeoutSec:         timeout,
		RetryMax:           retry,
		IsActive:           active,
	}
	return out, nil
}

// ValidateTriggerSend — body POST /v1/channels/send.
func ValidateTriggerSend(req *dto.TriggerSendRequest) (int, error) {
	if req == nil || req.MaxPos == 0 {
		return 0, nil
	}
	if req.MaxPos < 0 || req.MaxPos > constants.LimitMax {
		return 0, errorspkg.ErrBadRequest.WithMessage(
			fmt.Sprintf("max_pos must be in [1, %d]", constants.LimitMax))
	}
	return req.MaxPos, nil
}
