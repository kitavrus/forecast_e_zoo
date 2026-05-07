package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

// ExportsStorageAPI — узкий интерфейс для cleanup-job (без import циклов).
type ExportsStorageAPI interface {
	ListExpired(ctx context.Context, before time.Time) ([]uuid.UUID, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// RegisterExportsCleanup регистрирует job-чистильщик: каждые 30 минут удаляет
// экспорты, у которых meta.CreatedAt < now - ttl.
func RegisterExportsCleanup(s gocron.Scheduler, storage ExportsStorageAPI, ttl time.Duration, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	job := func() {
		ctx := context.Background()
		before := time.Now().Add(-ttl)
		ids, err := storage.ListExpired(ctx, before)
		if err != nil {
			logger.WarnContext(ctx, "exports.cleanup.list_failed", slog.Any("error", err))
			return
		}
		for _, id := range ids {
			if err := storage.Delete(ctx, id); err != nil {
				logger.WarnContext(ctx, "exports.cleanup.delete_failed",
					slog.String("id", id.String()), slog.Any("error", err))
				continue
			}
		}
		if len(ids) > 0 {
			logger.InfoContext(ctx, "exports.cleanup.deleted", slog.Int("count", len(ids)))
		}
	}
	_, err := s.NewJob(
		gocron.DurationJob(30*time.Minute),
		gocron.NewTask(job),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	return err
}

// CleanupOnce — синхронный helper для тестов (не зависит от gocron).
func CleanupOnce(ctx context.Context, storage ExportsStorageAPI, ttl time.Duration, logger *slog.Logger) (int, error) {
	if logger == nil {
		logger = slog.Default()
	}
	before := time.Now().Add(-ttl)
	ids, err := storage.ListExpired(ctx, before)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, id := range ids {
		if err := storage.Delete(ctx, id); err != nil {
			logger.WarnContext(ctx, "exports.cleanup.delete_failed",
				slog.String("id", id.String()), slog.Any("error", err))
			continue
		}
		deleted++
	}
	return deleted, nil
}
