package service

import (
	"context"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
)

// Service — orchestrator поверх MartReader.
//
// Сейчас slim — просто делегирует в reader. Существует на случай будущих
// бизнес-правил (rate-limiting, audit, multi-source consolidation).
type Service struct {
	reader MartReader
}

// New создаёт Service с указанной MartReader (DI seam).
func New(reader MartReader) *Service { return &Service{reader: reader} }

// List — все mart'ы + версии.
func (s *Service) List(ctx context.Context) ([]models.MartInfo, error) {
	return s.reader.List(ctx)
}

// Read — стрим одной страницы строк mart'а.
func (s *Service) Read(
	ctx context.Context, name, cursorEnc string, limit int,
) ([]models.MartRow, string, models.MartVersion, error) {
	return s.reader.Read(ctx, name, cursorEnc, limit)
}

// GetVersion — текущая committed-версия mart'а.
func (s *Service) GetVersion(ctx context.Context, name string) (models.MartVersion, error) {
	return s.reader.GetVersion(ctx, name)
}

// GetSchema — schema mart'а.
func (s *Service) GetSchema(ctx context.Context, name string) (models.MartSchema, error) {
	return s.reader.GetSchema(ctx, name)
}
