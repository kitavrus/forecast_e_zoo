package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/mappers"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// GetMart — GET /v1/marts/:name?cursor=&limit=.
//
// Стримит NDJSON (application/x-ndjson, одна строка = один JSON-объект).
// Headers:
//   - X-Etl-Run-Id: <uuid>      — текущая версия mart'а в этом snapshot
//   - X-Next-Cursor: <base64>   — opaque cursor следующей страницы (если есть)
//   - Content-Type: application/x-ndjson
func (h *Handler) GetMart(c fiber.Ctx) error {
	name := c.Params("name")
	if !constants.IsKnownMart(name) {
		return errorspkg.WriteJSON(c, errorspkg.ErrNotFound.WithMessage("mart not found: "+name))
	}

	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	cursor := c.Query("cursor")

	rows, nextCursor, version, err := h.svc.Read(c.Context(), name, cursor, limit)
	if err != nil {
		return mappers.MapServiceError(c, err)
	}

	c.Set("X-Etl-Run-Id", version.EtlRunID.String())
	if nextCursor != "" {
		c.Set("X-Next-Cursor", nextCursor)
	}
	c.Set(fiber.HeaderContentType, "application/x-ndjson")
	c.Status(fiber.StatusOK)

	return streamRows(c, rows)
}

// streamRows — пишет []MartRow в response как NDJSON.
//
// Используем bufio.Writer вокруг fiber response writer для batch flush.
//
//nolint:wrapcheck // ошибки writer'а возвращаем как есть — Fiber их не оборачивает.
func streamRows(c fiber.Ctx, rows []models.MartRow) error {
	w := c.Response().BodyWriter()
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("data_marts: ndjson encode: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("data_marts: ndjson flush: %w", err)
	}
	return nil
}

// parseLimit — валидация query ?limit=.
func parseLimit(s string) (int, error) {
	if s == "" {
		return constants.LimitDefault, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, errorspkg.ErrBadRequest.WithMessage("invalid limit")
	}
	if n > constants.LimitMax {
		return 0, errorspkg.ErrBadRequest.WithMessage("limit exceeds max")
	}
	return n, nil
}
