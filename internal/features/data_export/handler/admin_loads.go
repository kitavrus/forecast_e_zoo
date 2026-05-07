// Package handler — HTTP-обработчики (Fiber v3) для source-adapter.
package handler

import (
	"context"
	"encoding/binary"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// AdminLoadsTrigger — интерфейс scheduler-а для admin-handler.
type AdminLoadsTrigger interface {
	TriggerOnce(ctx context.Context) error
}

// LoadsRepoAPI — узкий интерфейс repository для admin-handler.
type LoadsRepoAPI interface {
	GetByID(ctx context.Context, id uuid.UUID) (models.Load, error)
	GetRunning(ctx context.Context) (*models.Load, error)
}

// AdminLoadsHandler — handler группы /admin/loads*.
type AdminLoadsHandler struct {
	repo      LoadsRepoAPI
	trigger   AdminLoadsTrigger
	rejects   RejectRepoAPI
}

// RejectRepoAPI — интерфейс repository для /admin/reject-log.
type RejectRepoAPI interface {
	SelectRejects(ctx context.Context, f repository.RejectFilter, afterPK string, limit int) ([]repository.RejectRow, error)
}

// NewAdminLoadsHandler — конструктор.
func NewAdminLoadsHandler(repo LoadsRepoAPI, trigger AdminLoadsTrigger, rejects RejectRepoAPI) *AdminLoadsHandler {
	return &AdminLoadsHandler{repo: repo, trigger: trigger, rejects: rejects}
}

// PostLoads — POST /admin/loads. Стартует новый load асинхронно.
// Если уже running → 409 ErrLoadAlreadyRunning.
func (h *AdminLoadsHandler) PostLoads(c fiber.Ctx) error {
	var req dto.PostLoadRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.Wrap(err))
	}
	ctx := c.Context()
	running, err := h.repo.GetRunning(ctx)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	if running != nil {
		// 409 + currentLoadId в details.
		return errorspkg.WriteJSON(c, errorspkg.ErrLoadAlreadyRunning.WithDetails(errorspkg.Detail{
			Field:   "currentLoadId",
			Message: running.ID.String(),
		}))
	}
	// Async trigger (best-effort). gocron singleton mode защитит от race.
	go func() {
		// background-задача после ответа: фоновый ctx намеренный
		// (c.Context() будет cancelled сразу после возврата хендлера и убьёт TriggerOnce).
		_ = h.trigger.TriggerOnce(context.Background())
	}()
	resp := dto.PostLoadResponse{
		LoadID: uuid.Nil, // load_id появится после tick; клиент полит GET /admin/loads
		Status: "accepted",
	}
	return c.Status(fiber.StatusAccepted).JSON(resp)
}

// GetLoadByID — GET /admin/loads/{id}.
func (h *AdminLoadsHandler) GetLoadByID(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.WithMessage("invalid load id"))
	}
	load, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	resp := dto.LoadResponse{
		ID:            load.ID,
		StartedAt:     load.StartedAt,
		FinishedAt:    load.FinishedAt,
		Status:        string(load.Status),
		Source:        load.Source,
		FailureReason: load.FailureReason,
		ParentLoadID:  load.ParentLoadID,
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// PostLoadsRetry — POST /admin/loads/{id}/retry.
// MVP: проверяем что оригинальный load существует; стартуем новый load (как обычный).
func (h *AdminLoadsHandler) PostLoadsRetry(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.WithMessage("invalid load id"))
	}
	original, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	if original.Status != models.LoadStatusFailed {
		return errorspkg.WriteJSON(c, errorspkg.ErrCannotRetry.WithMessage("only failed loads can be retried"))
	}
	go func() {
		// background-задача после ответа: фоновый ctx намеренный
		// (c.Context() будет cancelled сразу после возврата хендлера и убьёт TriggerOnce).
		_ = h.trigger.TriggerOnce(context.Background())
	}()
	return c.Status(fiber.StatusAccepted).JSON(dto.PostLoadRetryResponse{
		NewLoadID:      uuid.Nil, // появится после tick
		OriginalLoadID: original.ID,
	})
}

// GetRejectLog — GET /admin/reject-log.
func (h *AdminLoadsHandler) GetRejectLog(c fiber.Ctx) error {
	loadIDStr := c.Query("load_id")
	entity := c.Query("entity")
	severity := c.Query("severity")
	cursor := c.Query("cursor")
	limitStr := c.Query("limit")

	limit := 1000
	if limitStr != "" {
		l, err := strconvAtoiSafe(limitStr)
		if err != nil || l <= 0 || l > 10000 {
			return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.WithMessage("invalid limit"))
		}
		limit = l
	}

	var lid uuid.UUID
	if loadIDStr != "" {
		parsed, err := uuid.Parse(loadIDStr)
		if err != nil {
			return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.WithMessage("invalid load_id"))
		}
		lid = parsed
	}

	rows, err := h.rejects.SelectRejects(c.Context(),
		repository.RejectFilter{LoadID: lid, Entity: entity, Severity: severity},
		cursor, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	items := make([]dto.RejectLogEntry, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.RejectLogEntry{
			ID:       uuidFromInt64(r.ID),
			LoadID:   r.LoadID,
			Entity:   r.Entity,
			PKValue:  r.Payload,
			Severity: r.Severity,
			Reason:   "",
			Raw:      r.Errors,
		})
	}
	resp := dto.GetRejectLogResponse{Items: items}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// _ helpers ------------------------------------------------------------------

func strconvAtoiSafe(s string) (int, error) {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errorspkg.ErrBadRequest
		}
		n = n*10 + int(ch-'0')
		if n < 0 {
			return 0, errorspkg.ErrBadRequest
		}
	}
	return n, nil
}

// uuidFromInt64 — синтетический uuid из bigint id (для DTO-совместимости).
// reject_log.id = bigserial, наш DTO ожидает uuid; первые 8 байт — big-endian id.
func uuidFromInt64(n int64) uuid.UUID {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n)) //nolint:gosec // semantic reinterpret
	u := uuid.UUID{}
	copy(u[:8], buf[:])
	return u
}

