// Package handler — Fiber v3 хендлеры фичи forecast (Module 5).
//
// Один action = один файл (snake_case): list_runs.go, get_run.go,
// refresh_run.go, list_plans.go, get_plan.go, approve_plan.go.
package handler

import (
	"context"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// ForecastService — узкий interface (DI seam).
type ForecastService interface {
	ListRuns(ctx context.Context, f models.RunFilter) ([]models.ForecastRun, string, error)
	GetRun(ctx context.Context, id uuid.UUID) (models.ForecastRun, error)
	TriggerRefresh(ctx context.Context, horizonDays int) (uuid.UUID, bool, error)
	ListPlans(ctx context.Context, f models.PlanFilter) ([]models.ReplenishmentPlan, string, error)
	GetPlanWithLines(ctx context.Context, id uuid.UUID) (models.PlanWithLines, error)
	ApprovePlan(ctx context.Context, id uuid.UUID, approvedBy string) (models.ReplenishmentPlan, error)
}

// Handler — все forecast endpoints.
type Handler struct {
	svc ForecastService
}

// NewHandler создаёт Handler.
func NewHandler(svc ForecastService) *Handler { return &Handler{svc: svc} }
