// Package handler — Fiber v3 хендлеры фичи kpi.
//
// Один action = один файл (snake_case): list_snapshots.go, get_snapshot.go,
// list_calibrations.go, update_calibration.go, refresh_snapshots.go.
package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

// KpiService — узкий interface (DI seam, testability).
type KpiService interface {
	ListSnapshots(ctx context.Context, f models.SnapshotFilter) ([]models.KpiSnapshot, string, error)
	GetSnapshot(ctx context.Context, id uuid.UUID) (models.KpiSnapshot, error)
	ListCalibrations(ctx context.Context, f models.CalibrationFilter) ([]models.KpiCalibration, error)
	UpdateCalibration(ctx context.Context, id uuid.UUID, params json.RawMessage) (models.KpiCalibration, error)
	TriggerRefresh(ctx context.Context, asOfDate time.Time, kpiNames []string) (uuid.UUID, bool, error)
}

// Handler — все KPI endpoints.
type Handler struct {
	svc KpiService
}

// NewHandler создаёт Handler.
func NewHandler(svc KpiService) *Handler { return &Handler{svc: svc} }
