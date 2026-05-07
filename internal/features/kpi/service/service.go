// Package service — бизнес-логика поверх repository и engine/scheduler.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

// Repo — узкий интерфейс репозитория для service.
type Repo interface {
	ListSnapshots(ctx context.Context, f models.SnapshotFilter) ([]models.KpiSnapshot, string, error)
	GetSnapshotByID(ctx context.Context, id uuid.UUID) (models.KpiSnapshot, error)
	ListCalibrations(ctx context.Context, f models.CalibrationFilter) ([]models.KpiCalibration, error)
	GetCalibrationByID(ctx context.Context, id uuid.UUID) (models.KpiCalibration, error)
	UpdateCalibration(ctx context.Context, id uuid.UUID, params json.RawMessage) (models.KpiCalibration, error)
}

// Trigger — интерфейс scheduler-а для refresh endpoint-а.
type Trigger interface {
	TryTrigger(ctx context.Context, asOfDate time.Time, kpiNames []string) (uuid.UUID, bool, error)
}

// Service — orchestrator для handlers.
type Service struct {
	repo    Repo
	trigger Trigger
}

// New создаёт Service.
func New(repo Repo, trigger Trigger) *Service {
	return &Service{repo: repo, trigger: trigger}
}

// ListSnapshots — pass-through.
func (s *Service) ListSnapshots(
	ctx context.Context, f models.SnapshotFilter,
) ([]models.KpiSnapshot, string, error) {
	return s.repo.ListSnapshots(ctx, f) //nolint:wrapcheck // repo уже оборачивает
}

// GetSnapshot — pass-through.
func (s *Service) GetSnapshot(ctx context.Context, id uuid.UUID) (models.KpiSnapshot, error) {
	return s.repo.GetSnapshotByID(ctx, id) //nolint:wrapcheck
}

// ListCalibrations — pass-through.
func (s *Service) ListCalibrations(
	ctx context.Context, f models.CalibrationFilter,
) ([]models.KpiCalibration, error) {
	return s.repo.ListCalibrations(ctx, f) //nolint:wrapcheck
}

// UpdateCalibration — обновление params.
func (s *Service) UpdateCalibration(
	ctx context.Context, id uuid.UUID, params json.RawMessage,
) (models.KpiCalibration, error) {
	return s.repo.UpdateCalibration(ctx, id, params) //nolint:wrapcheck
}

// TriggerRefresh — POST /v1/kpi/snapshots/refresh.
//
// Если scheduler не настроен (Trigger=nil) — возвращает ошибку 503.
func (s *Service) TriggerRefresh(
	ctx context.Context, asOfDate time.Time, kpiNames []string,
) (uuid.UUID, bool, error) {
	if s.trigger == nil {
		return uuid.Nil, false, fmt.Errorf("kpi service: scheduler not configured")
	}
	id, started, err := s.trigger.TryTrigger(ctx, asOfDate, kpiNames)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("kpi service: trigger refresh: %w", err)
	}
	return id, started, nil
}
