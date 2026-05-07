// Package service содержит бизнес-логику фичи data_marts:
//   - MartReader interface — abstraction поверх storage (DI seam, ADR-005)
//   - PGReader — реализация поверх repository (PG)
//   - Cache — in-memory cache версий (60s TTL, ADR-004)
//   - Service — orchestrator, который handler-ы используют
package service

import (
	"context"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
)

// MartReader — abstract storage для mart-данных.
//
// Реализации:
//   - PGReader (текущая, поверх pgx + marts.* schema)
//   - в будущем: ClickHouseReader, ParquetReader, DuckDBReader (см. ADR-005)
type MartReader interface {
	// List возвращает per-mart info со списком текущих версий.
	List(ctx context.Context) ([]models.MartInfo, error)

	// Read стримит страницу строк mart'а по cursor.
	// Возвращает rows + nextCursor (opaque base64; пустой если последняя страница).
	Read(ctx context.Context, name string, cursor string, limit int) (
		rows []models.MartRow, nextCursor string, version models.MartVersion, err error)

	// GetVersion возвращает текущую committed-версию mart'а.
	GetVersion(ctx context.Context, name string) (models.MartVersion, error)

	// GetSchema возвращает схему mart'а (поля + типы).
	GetSchema(ctx context.Context, name string) (models.MartSchema, error)
}
