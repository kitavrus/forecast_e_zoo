package exports_storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// LocalFSStorage — Local FS реализация ExportsStorage.
// Layout: {root}/{id}.{format} + {root}/{id}.meta.json.
type LocalFSStorage struct {
	root string
}

// NewLocalFS создаёт LocalFSStorage. Root создаётся, если не существует.
func NewLocalFS(root string) (*LocalFSStorage, error) {
	if root == "" {
		return nil, errors.New("exports_storage: empty root")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("exports_storage: mkdir root: %w", err)
	}
	return &LocalFSStorage{root: root}, nil
}

// Root возвращает корневой путь (для DI/тестов).
func (s *LocalFSStorage) Root() string { return s.root }

func (s *LocalFSStorage) bodyPath(id uuid.UUID, format string) string {
	return filepath.Join(s.root, id.String()+"."+strings.ToLower(format))
}

func (s *LocalFSStorage) metaPath(id uuid.UUID) string {
	return filepath.Join(s.root, id.String()+".meta.json")
}

// Put сохраняет body и meta. Возвращает абсолютный путь к body-файлу.
func (s *LocalFSStorage) Put(_ context.Context, id uuid.UUID, format string, body io.Reader, meta Meta) (string, error) {
	bodyP := s.bodyPath(id, format)
	f, err := os.Create(bodyP) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("exports_storage: create body: %w", err)
	}
	n, copyErr := io.Copy(f, body)
	closeErr := f.Close()
	if copyErr != nil {
		return "", fmt.Errorf("exports_storage: write body: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("exports_storage: close body: %w", closeErr)
	}
	meta.SizeBytes = n
	meta.Format = format
	if meta.Status == "" {
		meta.Status = "ready"
	}
	if err := s.writeMeta(id, meta); err != nil {
		return "", err
	}
	return bodyP, nil
}

// PutMeta — обновляет только meta.
func (s *LocalFSStorage) PutMeta(_ context.Context, id uuid.UUID, meta Meta) error {
	return s.writeMeta(id, meta)
}

func (s *LocalFSStorage) writeMeta(id uuid.UUID, meta Meta) error {
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("exports_storage: marshal meta: %w", err)
	}
	return os.WriteFile(s.metaPath(id), raw, 0o600)
}

// Get возвращает путь к body + meta.
func (s *LocalFSStorage) Get(_ context.Context, id uuid.UUID) (string, Meta, error) {
	metaRaw, err := os.ReadFile(s.metaPath(id)) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", Meta{}, errorspkg.ErrExportNotFound
		}
		return "", Meta{}, fmt.Errorf("exports_storage: read meta: %w", err)
	}
	var meta Meta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return "", Meta{}, fmt.Errorf("exports_storage: parse meta: %w", err)
	}
	return s.bodyPath(id, meta.Format), meta, nil
}

// Delete удаляет body и meta идемпотентно.
func (s *LocalFSStorage) Delete(ctx context.Context, id uuid.UUID) error {
	_, meta, err := s.Get(ctx, id)
	// Если meta нет — считаем удаление успешным (идемпотентно).
	if errors.Is(err, errorspkg.ErrExportNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	bodyP := s.bodyPath(id, meta.Format)
	_ = os.Remove(bodyP)
	_ = os.Remove(s.metaPath(id))
	return nil
}

// ListExpired сканирует root и возвращает id, у которых meta.CreatedAt < before.
func (s *LocalFSStorage) ListExpired(_ context.Context, before time.Time) ([]uuid.UUID, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("exports_storage: read dir: %w", err)
	}
	out := make([]uuid.UUID, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		idStr := strings.TrimSuffix(e.Name(), ".meta.json")
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.root, e.Name())) //nolint:gosec
		if err != nil {
			continue
		}
		var meta Meta
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}
		if meta.CreatedAt.Before(before) {
			out = append(out, id)
		}
	}
	return out, nil
}

// compile-time check.
var _ ExportsStorage = (*LocalFSStorage)(nil)
