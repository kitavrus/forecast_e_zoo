// Package service — бизнес-логика поверх repository и scheduler фичи forecast.
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Repo — узкий интерфейс репозитория для service.
type Repo interface {
	ListRuns(ctx context.Context, f models.RunFilter) ([]models.ForecastRun, string, error)
	GetRunByID(ctx context.Context, id uuid.UUID) (models.ForecastRun, error)

	ListPlans(ctx context.Context, f models.PlanFilter) ([]models.ReplenishmentPlan, string, error)
	GetPlanByID(ctx context.Context, id uuid.UUID) (models.ReplenishmentPlan, error)
	GetPlanLines(ctx context.Context, runID uuid.UUID, supplierID, locationID string) ([]models.CalculationLine, error)
	ApprovePlan(ctx context.Context, id uuid.UUID, approvedBy string) (models.ReplenishmentPlan, error)
}

// Trigger — interface scheduler-а для refresh endpoint-а.
type Trigger interface {
	TryTrigger(ctx context.Context, horizonDays int) (uuid.UUID, bool, error)
}

// Service — orchestrator для handlers.
type Service struct {
	repo    Repo
	trigger Trigger
}

// New собирает Service.
func New(repo Repo, trigger Trigger) *Service {
	return &Service{repo: repo, trigger: trigger}
}

// ListRuns — pass-through.
func (s *Service) ListRuns(ctx context.Context, f models.RunFilter) ([]models.ForecastRun, string, error) {
	return s.repo.ListRuns(ctx, f) //nolint:wrapcheck
}

// GetRun — pass-through.
func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (models.ForecastRun, error) {
	return s.repo.GetRunByID(ctx, id) //nolint:wrapcheck
}

// TriggerRefresh — POST /v1/forecast/runs/refresh.
func (s *Service) TriggerRefresh(ctx context.Context, horizonDays int) (uuid.UUID, bool, error) {
	if s.trigger == nil {
		return uuid.Nil, false, errorspkg.ErrForecastSchedulerUnavailable
	}
	id, started, err := s.trigger.TryTrigger(ctx, horizonDays)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("forecast service: trigger refresh: %w", err)
	}
	return id, started, nil
}

// ListPlans — pass-through.
func (s *Service) ListPlans(ctx context.Context, f models.PlanFilter) ([]models.ReplenishmentPlan, string, error) {
	return s.repo.ListPlans(ctx, f) //nolint:wrapcheck
}

// GetPlanWithLines — план + все его lines.
func (s *Service) GetPlanWithLines(ctx context.Context, id uuid.UUID) (models.PlanWithLines, error) {
	p, err := s.repo.GetPlanByID(ctx, id)
	if err != nil {
		return models.PlanWithLines{}, err //nolint:wrapcheck
	}
	lines, err := s.repo.GetPlanLines(ctx, p.RunID, p.SupplierID, p.LocationID)
	if err != nil {
		return models.PlanWithLines{}, fmt.Errorf("forecast service: get plan lines: %w", err)
	}
	return models.PlanWithLines{Plan: p, Lines: lines}, nil
}

// ApprovePlan — переводит plan в approved.
func (s *Service) ApprovePlan(
	ctx context.Context, id uuid.UUID, approvedBy string,
) (models.ReplenishmentPlan, error) {
	return s.repo.ApprovePlan(ctx, id, approvedBy) //nolint:wrapcheck
}
