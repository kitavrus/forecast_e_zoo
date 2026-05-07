// Package exports_storage — хранение результатов /v1/exports.
// MVP-реализация — local FS (LocalFSStorage). Для S3-варианта добавится
// отдельная реализация без изменения интерфейса.
package exports_storage

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// Meta — метаданные экспорта (хранится рядом с файлом в .meta.json).
type Meta struct {
	Entity     string    `json:"entity"`
	Format     string    `json:"format"`
	SnapshotID string    `json:"snapshot_id"`
	Requester  string    `json:"requester"`
	CreatedAt  time.Time `json:"created_at"`
	SizeBytes  int64     `json:"size_bytes"`
	Status     string    `json:"status"` // pending | ready | failed
	Error      string    `json:"error,omitempty"`
}

// ExportsStorage — порт хранения экспортов.
type ExportsStorage interface {
	// Put сохраняет тело файла + meta. Возвращает path сохранённого файла.
	Put(ctx context.Context, id uuid.UUID, format string, body io.Reader, meta Meta) (string, error)
	// Get возвращает path к файлу + meta. ErrExportNotFound если нет.
	Get(ctx context.Context, id uuid.UUID) (string, Meta, error)
	// PutMeta — обновляет только meta (без body). Используется при
	// async-pipeline pending → ready.
	PutMeta(ctx context.Context, id uuid.UUID, meta Meta) error
	// Delete удаляет файл и meta (идемпотентно).
	Delete(ctx context.Context, id uuid.UUID) error
	// ListExpired возвращает id экспортов, у которых CreatedAt < before.
	ListExpired(ctx context.Context, before time.Time) ([]uuid.UUID, error)
}
