package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// MasterChangeLogRepoAPI — узкий интерфейс repository для /v1/master_change_log.
type MasterChangeLogRepoAPI interface {
	SelectMasterChangeLog(ctx context.Context, loadID uuid.UUID, afterPK string, limit int) ([]repository.MasterChangeLogReadRow, error)
}

// MasterChangeLogHandler — GET /v1/master_change_log.
type MasterChangeLogHandler struct {
	repo MasterChangeLogRepoAPI
	snap SnapshotProvider
}

// NewMasterChangeLogHandler — конструктор.
func NewMasterChangeLogHandler(repo MasterChangeLogRepoAPI, snap SnapshotProvider) *MasterChangeLogHandler {
	return &MasterChangeLogHandler{repo: repo, snap: snap}
}

type masterChangeLogStreamItem struct {
	ID        int64           `json:"id"`
	Entity    string          `json:"entity"`
	EntityPK  json.RawMessage `json:"entity_pk"`
	Field     string          `json:"field"`
	OldValue  json.RawMessage `json:"old_value,omitempty"`
	NewValue  json.RawMessage `json:"new_value,omitempty"`
	ChangedAt time.Time       `json:"changed_at"`
}

// Get — GET /v1/master_change_log?cursor=&limit=.
func (h *MasterChangeLogHandler) Get(c fiber.Ctx) error {
	cursor, err := validators.ParseCursor(c.Query("cursor"))
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}
	limit, err := validators.ParseLimit(c.Query("limit"), dto.LimitDefault, dto.LimitMax)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	sp, err := h.snap.Current(c.Context())
	if err != nil {
		c.Set("Retry-After", "60")
		return errorspkg.WriteJSON(c, err)
	}
	if sp.CurrentLoadID == nil {
		c.Set("Retry-After", "60")
		return errorspkg.WriteJSON(c, errorspkg.ErrSnapshotNotReady)
	}
	loadID := *sp.CurrentLoadID

	rows, err := h.repo.SelectMasterChangeLog(c.Context(), loadID, cursor.AfterPK, limit)
	if err != nil {
		return errorspkg.WriteJSON(c, err)
	}

	etag := ComputeETag(loadID, "master_change_log", derefOrZeroTime(sp.CommittedAt))
	if CheckIfNoneMatch(c, etag) {
		WritePageHeaders(c, loadID, loadID, etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	WritePageHeaders(c, loadID, loadID, etag)
	// X-Next-Cursor: если страница «полная» (len == limit), вероятно есть продолжение.
	if len(rows) == limit && limit > 0 {
		last := rows[len(rows)-1]
		WriteNextCursor(c, loadID, fmt.Sprintf("%d", last.ID))
	}

	items := make([]masterChangeLogStreamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, masterChangeLogStreamItem{
			ID:        r.ID,
			Entity:    r.Entity,
			EntityPK:  json.RawMessage(r.EntityPK),
			Field:     r.Field,
			OldValue:  json.RawMessage(r.OldValue),
			NewValue:  json.RawMessage(r.NewValue),
			ChangedAt: r.ChangedAt,
		})
	}
	return StreamNDJSON(c, items)
}
