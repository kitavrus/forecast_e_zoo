// Package snapshot — тонкий сервис над repository для snapshot-операций.
// Инкапсулирует транзакционный flip и единое место возврата ErrSnapshotNotReady.
package snapshot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
)

// SnapshotRepoAPI — узкий интерфейс repository для snapshot-сервиса.
type SnapshotRepoAPI interface {
	GetCurrent(ctx context.Context) (models.SnapshotPointer, error)
	Flip(ctx context.Context, tx pgx.Tx, loadID uuid.UUID) (models.SnapshotPointer, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// Service — публичный сервис.
type Service struct {
	repo   SnapshotRepoAPI
	logger *slog.Logger
}

// New создаёт Service.
func New(repo SnapshotRepoAPI, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, logger: logger}
}

// Current — возвращает текущий snapshot_pointer.
// Если snapshot не готов (нет committed load) — ErrSnapshotNotReady.
func (s *Service) Current(ctx context.Context) (models.SnapshotPointer, error) {
	return s.repo.GetCurrent(ctx)
}

// Flip — атомарно меняет current_load_id (вне loader-а; обычно вызывается loader-ом
// вместе с MarkCommitted в одной tx, но Service.Flip — для прямых вызовов admin-handler).
func (s *Service) Flip(ctx context.Context, loadID uuid.UUID) (models.SnapshotPointer, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.SnapshotPointer{}, fmt.Errorf("snapshot: begin tx: %w", err)
	}
	sp, err := s.repo.Flip(ctx, tx, loadID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return models.SnapshotPointer{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.SnapshotPointer{}, fmt.Errorf("snapshot: commit: %w", err)
	}
	s.logger.InfoContext(ctx, "snapshot.flipped", slog.String("load_id", loadID.String()))
	return sp, nil
}
