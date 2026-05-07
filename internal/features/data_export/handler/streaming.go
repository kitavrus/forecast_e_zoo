package handler

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// HeaderSnapshotID — выставляется на каждый /v1/* response.
const (
	HeaderSnapshotID = "X-Snapshot-Id"
	HeaderLoadID     = "X-Load-Id"
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
