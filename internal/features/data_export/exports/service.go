// Package exports — сервис обработки async-экспортов /v1/exports.
package exports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/exports_storage"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Service — оркестрирует экспорты.
type Service struct {
	storage exports_storage.ExportsStorage
	logger  *slog.Logger
}

// New создаёт Service.
func New(storage exports_storage.ExportsStorage, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{storage: storage, logger: logger}
}

// Start — стартует экспорт. Возвращает exportId сразу.
//
// Реальная сборка содержимого blocked: для master-сущностей нужны select-методы
// repository (фаза post-MVP — сейчас есть только products + receipt_line).
// Для MVP пишем pending → ready с пустым телом сразу — это разблокирует
// клиентов, желающих видеть жизненный цикл export-а.
func (s *Service) Start(ctx context.Context, req dto.PostExportRequest, requester string) (uuid.UUID, error) {
	if req.Format != "ndjson" && req.Format != "parquet" {
		return uuid.Nil, errorspkg.ErrInvalidExportFormat.WithMessage(
			fmt.Sprintf("format=%q not supported", req.Format))
	}
	id := uuid.New()
	meta := exports_storage.Meta{
		Entity:     req.Entity,
		Format:     req.Format,
		SnapshotID: req.SnapshotID.String(),
		Requester:  requester,
		CreatedAt:  time.Now().UTC(),
		Status:     "pending",
	}
	// Записываем pending-meta сразу: clients могут polling-ить GET.
	if err := s.storage.PutMeta(ctx, id, meta); err != nil {
		return uuid.Nil, fmt.Errorf("exports.Start: pending meta: %w", err)
	}
	// Async-job переживает request: используем background context осознанно.
	go s.runAsync(id, req, meta) //nolint:gosec // G118: detached background ctx by design
	return id, nil
}

// Get возвращает meta. ErrExportNotFound если нет.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (string, exports_storage.Meta, error) {
	return s.storage.Get(ctx, id)
}

// runAsync — для MVP пишет stub-body (пустой NDJSON) и помечает ready.
// В пост-MVP здесь будет потоковая запись через SourceReader/Repository.
func (s *Service) runAsync(id uuid.UUID, req dto.PostExportRequest, meta exports_storage.Meta) {
	ctx := context.Background()
	body := emptyBody(req.Entity)
	meta.Status = "ready"
	if _, err := s.storage.Put(ctx, id, req.Format, body, meta); err != nil {
		s.logger.Error("exports.run_failed", slog.String("id", id.String()), slog.Any("error", err))
		meta.Status = "failed"
		meta.Error = err.Error()
		_ = s.storage.PutMeta(ctx, id, meta)
		return
	}
	s.logger.Info("exports.ready", slog.String("id", id.String()), slog.String("entity", req.Entity))
}

func emptyBody(entity string) *bytes.Buffer {
	// Stub-body. В пост-MVP здесь будет stream данных.
	stub := map[string]any{
		"_note":   "stub export — repository-методы для всех сущностей реализуются в пост-MVP",
		"entity":  entity,
		"version": 1,
	}
	raw, _ := json.Marshal(stub)
	return bytes.NewBuffer(append(raw, '\n'))
}
