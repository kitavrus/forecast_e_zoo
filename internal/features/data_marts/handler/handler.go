// Package handler — Fiber v3 хендлеры фичи data_marts.
//
// Один action = один файл (snake_case): list_marts.go, get_mart.go, get_version.go, get_schema.go.
// Сам Handler — struct с зависимостями и конструктор.
package handler

import (
	"context"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
)

// MartsService — узкий интерфейс для handler'ов (testability).
type MartsService interface {
	List(ctx context.Context) ([]models.MartInfo, error)
	Read(ctx context.Context, name, cursorEnc string, limit int) (
		[]models.MartRow, string, models.MartVersion, error)
	GetVersion(ctx context.Context, name string) (models.MartVersion, error)
	GetSchema(ctx context.Context, name string) (models.MartSchema, error)
}

// Handler — все mart endpoints.
type Handler struct {
	svc MartsService
}

// NewHandler создаёт Handler.
func NewHandler(svc MartsService) *Handler { return &Handler{svc: svc} }
