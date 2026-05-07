package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// EtlRunListInput — параметры List от handler-а.
type EtlRunListInput struct {
	Status     string
	Kind       string
	BeforeTime *time.Time
	Limit      int
}

// EtlRunService — admin CRUD-API над etl_runs.
type EtlRunService struct {
	repo     Repo
	pipeline *EtlPipeline
}

// NewEtlRunService — DI-конструктор.
func NewEtlRunService(repo Repo, pipeline *EtlPipeline) *EtlRunService {
	return &EtlRunService{repo: repo, pipeline: pipeline}
}

// TriggerRun запускает full ETL run по запросу admin-а.
//
// Возвращает:
//   - 202 Accepted + run при успехе;
//   - errorspkg.ErrEtlRunAlreadyRunning, если другой run выполняется.
func (s *EtlRunService) TriggerRun(ctx context.Context, requester string) (*models.EtlRun, error) {
	var rq *string
	if requester != "" {
		rq = &requester
	}
	return s.pipeline.TryStart(ctx, constants.TriggerAdmin, rq, nil)
}

// Retry повторяет ETL run на основании предыдущего failed/aborted.
//
//   - status NOT IN (failed, aborted) → ErrCannotRetryEtl.
//   - parent_run_id указывает на исходный run, trigger='retry'.
func (s *EtlRunService) Retry(ctx context.Context, runID uuid.UUID, requester string) (*models.EtlRun, error) {
	prev, err := s.repo.GetEtlRunByID(ctx, runID)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped via errorspkg
	}
	if prev.Status != constants.StatusFailed && prev.Status != constants.StatusAborted {
		return nil, errorspkg.ErrCannotRetryEtl.Wrap(
			fmt.Errorf("status=%s is not retryable", prev.Status),
		)
	}
	var rq *string
	if requester != "" {
		rq = &requester
	}
	return s.pipeline.TryStart(ctx, constants.TriggerRetry, rq, &runID)
}

// GetByID возвращает один run.
func (s *EtlRunService) GetByID(ctx context.Context, id uuid.UUID) (*models.EtlRun, error) {
	return s.repo.GetEtlRunByID(ctx, id)
}

// List возвращает страницу runs.
func (s *EtlRunService) List(ctx context.Context, in EtlRunListInput) ([]models.EtlRun, error) {
	return s.repo.ListEtlRuns(ctx, repository.EtlRunListFilter{
		Status:     in.Status,
		Kind:       in.Kind,
		BeforeTime: in.BeforeTime,
		Limit:      in.Limit,
	})
}
