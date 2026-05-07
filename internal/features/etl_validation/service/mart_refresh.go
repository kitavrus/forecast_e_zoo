package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/loader"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/transformer"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// MartRefreshService — ondemand-refresh одного mart (см. ADR-021).
type MartRefreshService struct {
	pool     Pool
	repo     Repo
	registry Registry
	loader   LoaderIface
	extr     Extractor
}

// NewMartRefreshService — DI.
func NewMartRefreshService(pool Pool, repo Repo, reg Registry, l LoaderIface, e Extractor) *MartRefreshService {
	return &MartRefreshService{pool: pool, repo: repo, registry: reg, loader: l, extr: e}
}

// Refresh запускает synchronous refresh одного mart.
//
// Контракт:
//   - name должен быть в constants.MartRefreshable;
//   - INSERT etl_runs (kind='mart_refresh', target_mart=name, trigger='admin');
//   - extract snapshot;
//   - вызвать loader.Apply с одним builder-ом;
//   - вернуть run.
func (s *MartRefreshService) Refresh(ctx context.Context, name, requester string) (*models.EtlRun, error) {
	if !slices.Contains(constants.MartRefreshable, name) {
		return nil, errorspkg.ErrMartRefreshNotSupported
	}
	builder, err := s.registry.BuilderByName(name)
	if err != nil {
		return nil, errorspkg.ErrMartRefreshNotSupported.Wrap(err)
	}

	snap, err := s.extr.GetCurrentSnapshot(ctx)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped
	}
	sourceLoadID, err := uuid.Parse(snap.LoadID)
	if err != nil {
		return nil, fmt.Errorf("service: parse source_load_id: %w", err)
	}

	target := name
	var rq *string
	if requester != "" {
		rq = &requester
	}
	run := &models.EtlRun{
		ID:           uuid.New(),
		StartedAt:    time.Now().UTC(),
		Status:       constants.StatusRunning,
		Kind:         constants.KindMartRefresh,
		TargetMart:   &target,
		Trigger:      constants.TriggerAdmin,
		Requester:    rq,
		SourceLoadID: &sourceLoadID,
	}
	if err := s.repo.InsertEtlRun(ctx, run); err != nil {
		return nil, fmt.Errorf("service: insert etl_run: %w", err)
	}

	_, err = s.loader.Apply(ctx, loader.ApplyParams{
		RunID:        run.ID,
		SourceLoadID: sourceLoadID,
		Builders:     []transformer.Builder{builder},
	})
	if err != nil {
		// Помечаем run как failed.
		s.markFailed(ctx, run.ID, err.Error())
		return run, fmt.Errorf("service: apply: %w", err)
	}

	// Loader сам обновил status='committed' внутри tx. Возвращаем свежий run.
	return s.repo.GetEtlRunByID(ctx, run.ID)
}

func (s *MartRefreshService) markFailed(ctx context.Context, runID uuid.UUID, reason string) {
	now := time.Now().UTC()
	_ = s.repo.UpdateEtlRunStatus(ctx, runID, repository.EtlRunStatusPatch{
		Status:        constants.StatusFailed,
		FinishedAt:    &now,
		FailureReason: &reason,
	})
}
