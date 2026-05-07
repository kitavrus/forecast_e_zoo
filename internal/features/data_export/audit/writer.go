// Package audit содержит запись audit_access для админских эндпоинтов.
// /v1/* запросы НЕ аудятся (см. ADR-014: взрыв объёма).
package audit

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/internal/middleware"
)

// AdminPathPrefix — какие пути аудятся.
const AdminPathPrefix = "/admin/"

// AuditRepoAPI — узкий интерфейс repository для writer-а.
type AuditRepoAPI interface {
	InsertAudit(ctx context.Context, in repository.AuditAccessInput) error
}

// Writer — пишет audit_access записи.
type Writer struct {
	repo   AuditRepoAPI
	logger *slog.Logger
}

// New создаёт Writer.
func New(repo AuditRepoAPI, logger *slog.Logger) *Writer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Writer{repo: repo, logger: logger}
}

// Middleware — Fiber-handler. После c.Next() пишет в audit_access ТОЛЬКО для /admin/*.
// Best-effort: при ошибке БД — log и продолжаем (запрос не валим).
func (w *Writer) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		err := c.Next()

		path := c.Path()
		if !strings.HasPrefix(path, AdminPathPrefix) {
			return err
		}

		role, sub := actorFromCtx(c)
		traceID, _ := c.Locals(middleware.LocalsTraceID).(string)

		ctx := c.Context()
		if writeErr := w.repo.InsertAudit(ctx, repository.AuditAccessInput{
			ActorRole: role,
			ActorSub:  sub,
			Method:    c.Method(),
			Path:      path,
			Status:    c.Response().StatusCode(),
			TraceID:   traceID,
		}); writeErr != nil {
			w.logger.WarnContext(ctx, "audit.insert_failed",
				slog.String("path", path),
				slog.Any("error", writeErr))
		}
		return err
	}
}

// Insert — прямой helper для admin-handler-ов, если нужно записать запись вручную.
func (w *Writer) Insert(ctx context.Context, in repository.AuditAccessInput) error {
	return w.repo.InsertAudit(ctx, in)
}

// actorFromCtx — извлекает role/subject из JWT-claims, лежащих в Locals
// (см. middleware.JWT). Если claims нет — возвращает пустые строки.
func actorFromCtx(c fiber.Ctx) (role, sub string) {
	claims, ok := middleware.ClaimsFromCtx(c)
	if !ok || claims == nil {
		return "", ""
	}
	return claims.Issuer, claims.Subject
}
