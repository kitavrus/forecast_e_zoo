package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/service"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validators"
	"github.com/Kitavrus/e_zoo/internal/middleware"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// PostEtlRun реализует POST /admin/etl-runs.
//
// @Summary Запустить ETL run (admin)
// @Tags admin-etl
// @Accept json
// @Produce json
// @Success 202 {object} dto.EtlRunResponse
// @Failure 409 {object} errorspkg.ErrorResponse
// @Router /admin/etl-runs [post]
func (h *Handler) PostEtlRun(c fiber.Ctx) error {
	if err := h.validator.ValidatePostEtlRun(validators.PostEtlRunInput{}); err != nil {
		return mappers.MapServiceError(c, err)
	}
	requester := requesterFromCtx(c)
	run, err := h.runs.TriggerRun(c.Context(), requester)
	if err != nil {
		return mappers.MapTriggerRunError(c, err)
	}
	return c.Status(http.StatusAccepted).JSON(toEtlRunResponse(run))
}

// RetryEtlRun реализует POST /admin/etl-runs/:id/retry.
func (h *Handler) RetryEtlRun(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.validator.ValidateRetryEtlRun(id); err != nil {
		return mappers.MapServiceError(c, err)
	}
	uid, _ := uuid.Parse(id)
	requester := requesterFromCtx(c)
	run, err := h.runs.Retry(c.Context(), uid, requester)
	if err != nil {
		return mappers.MapRetryError(c, err)
	}
	return c.Status(http.StatusAccepted).JSON(toEtlRunResponse(run))
}

// GetEtlRun реализует GET /admin/etl-runs/:id.
func (h *Handler) GetEtlRun(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.validator.ValidateGetEtlRun(id); err != nil {
		return mappers.MapServiceError(c, err)
	}
	uid, _ := uuid.Parse(id)
	run, err := h.runs.GetByID(c.Context(), uid)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	return c.JSON(toEtlRunResponse(run))
}

// ListEtlRuns реализует GET /admin/etl-runs?status=&kind=&cursor=&limit=.
func (h *Handler) ListEtlRuns(c fiber.Ctx) error {
	q := validators.ListEtlRunsQuery{
		Status: c.Query("status"),
		Kind:   c.Query("kind"),
		Cursor: c.Query("cursor"),
		Limit:  parseInt(c.Query("limit")),
	}
	if err := h.validator.ValidateListEtlRuns(q); err != nil {
		return mappers.MapServiceError(c, err)
	}
	in := service.EtlRunListInput{Status: q.Status, Kind: q.Kind, Limit: q.Limit}
	if q.Cursor != "" {
		t, _ := time.Parse(time.RFC3339, q.Cursor)
		in.BeforeTime = &t
	}
	runs, err := h.runs.List(c.Context(), in)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	out := dto.EtlRunListResponse{Items: make([]dto.EtlRunResponse, 0, len(runs))}
	for i := range runs {
		out.Items = append(out.Items, toEtlRunResponse(&runs[i]))
	}
	if len(runs) > 0 {
		out.NextCursor = runs[len(runs)-1].StartedAt.UTC().Format(time.RFC3339Nano)
	}
	return c.JSON(out)
}

// RefreshMart реализует POST /admin/marts/:name/refresh.
func (h *Handler) RefreshMart(c fiber.Ctx) error {
	name := c.Params("name")
	if err := h.validator.ValidateMartRefresh(name); err != nil {
		return mappers.MapMartRefreshError(c, err)
	}
	requester := requesterFromCtx(c)
	run, err := h.refresh.Refresh(c.Context(), name, requester)
	if err != nil {
		return mappers.MapMartRefreshError(c, err)
	}
	return c.Status(http.StatusAccepted).JSON(dto.MartRefreshResponse{
		RunID:      run.ID.String(),
		Status:     run.Status,
		TargetMart: name,
	})
}

// ListRejectLog реализует GET /admin/reject-log?etl_run_id=&entity=&severity=&cursor=&limit=.
func (h *Handler) ListRejectLog(c fiber.Ctx) error {
	q := validators.ListRejectLogQuery{
		EtlRunID: c.Query("etl_run_id"),
		Entity:   c.Query("entity"),
		Severity: c.Query("severity"),
		Cursor:   c.Query("cursor"),
		Limit:    parseInt(c.Query("limit")),
	}
	if err := h.validator.ValidateListRejectLog(q); err != nil {
		return mappers.MapServiceError(c, err)
	}
	// Build repository filter (avoid importing repository here would be ugly,
	// so we use the package import).
	filter := buildRejectFilter(q)
	entries, err := h.rejects.ListRejectEntries(c.Context(), filter)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}
	out := dto.RejectLogListResponse{Items: make([]dto.RejectLogEntryResponse, 0, len(entries))}
	for _, e := range entries {
		out.Items = append(out.Items, toRejectLogResponse(e))
	}
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		out.NextCursor = strconv.FormatInt(last.ID, 10)
	}
	return c.JSON(out)
}

// Healthz возвращает простой 200 + статус.
func (h *Handler) Healthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
	})
}

// requesterFromCtx — извлекает sub из JWT-claims (см. internal/middleware/jwt.go).
//
// Если claims отсутствуют (что не должно случаться после JWT middleware) или
// Subject пуст — возвращает пустую строку.
func requesterFromCtx(c fiber.Ctx) string {
	claims, ok := middleware.ClaimsFromCtx(c)
	if !ok || claims == nil {
		return ""
	}
	return claims.Subject
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// AssertSentinel — sanity-функция, чтобы errorspkg не выпадал из импорт-зависимости
// до фактического использования (компилятору нужно).
func AssertSentinel() error { return errorspkg.ErrEtlRunNotFound }
