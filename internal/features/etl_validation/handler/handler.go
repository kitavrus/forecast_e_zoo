// Package handler реализует HTTP-handlers admin-API фичи etl_validation.
package handler

import (
	"context"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/service"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validators"
)

// RejectLogReader — узкий контракт для GET /admin/reject-log (read-only).
type RejectLogReader interface {
	ListRejectEntries(ctx context.Context, f repository.RejectLogListFilter) ([]models.RejectLogEntry, error)
}

// Handler — DI-конструируемый struct, объединяющий services + validators.
type Handler struct {
	runs      *service.EtlRunService
	refresh   *service.MartRefreshService
	rejects   RejectLogReader
	validator validators.Validator
}

// NewHandler — DI-конструктор.
func NewHandler(
	runs *service.EtlRunService,
	refresh *service.MartRefreshService,
	rejects RejectLogReader,
	v validators.Validator,
) *Handler {
	return &Handler{runs: runs, refresh: refresh, rejects: rejects, validator: v}
}
