package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ComputeETag — детерминированная weak ETag для ответа.
// W/"<sha256>" — W/ как weak (для ETag/Cache-Control private).
func ComputeETag(loadID uuid.UUID, entity string, lastModified time.Time) string {
	h := sha256.New()
	_, _ = h.Write(loadID[:])
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(entity))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(lastModified.UTC().Format(time.RFC3339Nano)))
	return `W/"` + hex.EncodeToString(h.Sum(nil)) + `"`
}

// CheckIfNoneMatch — true, если клиент уже имеет данные (etag совпал).
// Тогда handler должен вернуть 304 Not Modified.
func CheckIfNoneMatch(c fiber.Ctx, etag string) bool {
	got := c.Get(fiber.HeaderIfNoneMatch)
	return got != "" && got == etag
}
