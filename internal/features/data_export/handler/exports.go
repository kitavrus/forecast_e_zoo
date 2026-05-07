package handler

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/exports_storage"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ExportsService — интерфейс для exports-handler.
type ExportsService interface {
	Start(ctx context.Context, req dto.PostExportRequest, requester string) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (path string, meta exports_storage.Meta, err error)
}

// ExportsHandler — HTTP-обработчики /v1/exports.
type ExportsHandler struct {
	svc ExportsService
}

// NewExportsHandler — конструктор.
func NewExportsHandler(svc ExportsService) *ExportsHandler { return &ExportsHandler{svc: svc} }

// Post — POST /v1/exports.
func (h *ExportsHandler) Post(c fiber.Ctx) error {
	var req dto.PostExportRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.Wrap(err))
	}
	v := validators.NewValidator()
	if err := v.Struct(req); err != nil {
		// validate.Struct can return many violations; здесь mapping короткий.
		// Для format специально — ErrInvalidExportFormat.
		if isFormatViolation(err) {
			return errorspkg.WriteJSON(c, errorspkg.ErrInvalidExportFormat.Wrap(err))
		}
		return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.Wrap(err))
	}

	requester := requesterFromCtx(c)
	id, err := h.svc.Start(c.Context(), req, requester)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	resp := dto.PostExportResponse{
		ExportID: id,
		Status:   "pending",
		Location: "/v1/exports/" + id.String(),
	}
	return c.Status(fiber.StatusAccepted).JSON(resp)
}

// Get — GET /v1/exports/{id}.
// 200 + redirect/stream если ready, 202 если pending, 404 если нет.
func (h *ExportsHandler) Get(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return errorspkg.WriteJSON(c, errorspkg.ErrBadRequest.WithMessage("invalid export id"))
	}
	path, meta, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	switch meta.Status {
	case "ready":
		// MVP: возвращаем JSON-meta с location. Реальное скачивание — через
		// Fiber static (см. фазу 15 router) на /files/exports/{id}.{format}.
		return c.Status(fiber.StatusOK).JSON(dto.GetExportResponse{
			ID:         id,
			Entity:     meta.Entity,
			SnapshotID: parseUUIDOrZero(meta.SnapshotID),
			Format:     meta.Format,
			Status:     meta.Status,
			Location:   &path,
			SizeBytes:  &meta.SizeBytes,
			CreatedAt:  meta.CreatedAt,
		})
	case "failed":
		errMsg := meta.Error
		return c.Status(fiber.StatusOK).JSON(dto.GetExportResponse{
			ID:        id,
			Entity:    meta.Entity,
			Format:    meta.Format,
			Status:    meta.Status,
			Error:     &errMsg,
			CreatedAt: meta.CreatedAt,
		})
	default: // pending
		return c.Status(fiber.StatusAccepted).JSON(dto.GetExportResponse{
			ID:        id,
			Entity:    meta.Entity,
			Format:    meta.Format,
			Status:    "pending",
			CreatedAt: meta.CreatedAt,
		})
	}
}

func requesterFromCtx(c fiber.Ctx) string {
	claims, ok := middleware.ClaimsFromCtx(c)
	if !ok || claims == nil {
		return ""
	}
	return claims.Issuer + ":" + claims.Subject
}

func parseUUIDOrZero(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func isFormatViolation(err error) bool {
	if err == nil {
		return false
	}
	var marker interface{ Error() string }
	return errors.As(err, &marker) && (containsAny(err.Error(), "Format", "format"))
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		for i := 0; i+len(x) <= len(s); i++ {
			if s[i:i+len(x)] == x {
				return true
			}
		}
	}
	return false
}
