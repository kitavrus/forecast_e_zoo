package handler

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
)

// HeaderSnapshotID — выставляется на каждый /v1/* response.
const (
	HeaderSnapshotID = "X-Snapshot-Id"
	HeaderLoadID     = "X-Load-Id"
	HeaderNextCursor = "X-Next-Cursor"
)

// WritePageHeaders — общие headers для read-handlers /v1/*.
func WritePageHeaders(c fiber.Ctx, snapshotID, loadID uuid.UUID, etag string) {
	if snapshotID != uuid.Nil {
		c.Set(HeaderSnapshotID, snapshotID.String())
	}
	if loadID != uuid.Nil {
		c.Set(HeaderLoadID, loadID.String())
	}
	if etag != "" {
		c.Set(fiber.HeaderETag, etag)
	}
	c.Set(fiber.HeaderCacheControl, "private, max-age=86400")
}

// WriteNextCursor — сериализует cursor (loadID + afterPK) в base64
// и выставляет X-Next-Cursor header. Используется handlers, когда rows
// заполнили весь limit (значит, есть продолжение).
//
// При afterPK == "" header не выставляется. При ошибке Encode header
// тоже пропускается — клиент остановит итерацию (один лишний reload не критичен).
func WriteNextCursor(c fiber.Ctx, loadID uuid.UUID, afterPK string) {
	if afterPK == "" {
		return
	}
	cur := models.Cursor{LoadID: loadID.String(), AfterPK: afterPK}
	enc, err := cur.Encode()
	if err != nil || enc == "" {
		return
	}
	c.Set(HeaderNextCursor, enc)
}

// StreamNDJSON — выставляет Content-Type: application/x-ndjson и пишет тело.
// Для MVP буферизуем — большие выгрузки идут через /v1/exports (фаза 14).
func StreamNDJSON[T any](c fiber.Ctx, items []T) error {
	c.Set(fiber.HeaderContentType, "application/x-ndjson")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return c.Status(fiber.StatusOK).SendStream(&buf)
}
