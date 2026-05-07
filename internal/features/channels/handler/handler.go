// Package handler — Fiber v3 хендлеры фичи channels (Module 7 channel-routing).
//
// Один action = один файл (snake_case): list_send_attempts.go, get_send_attempt.go,
// trigger_send.go, retry.go, list_configs.go, upsert_config.go.
package handler

import (
	"context"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

// ChannelService — узкий interface для DI seam.
type ChannelService interface {
	ListSendAttempts(ctx context.Context, f models.SendAttemptFilter) ([]models.SendAttempt, string, error)
	GetSendAttempt(ctx context.Context, id uuid.UUID) (models.SendAttempt, error)
	ListConfigs(ctx context.Context) ([]models.SupplierChannelConfig, error)
	UpsertConfig(ctx context.Context, in models.UpsertChannelConfigInput) (models.SupplierChannelConfig, error)
	TriggerSendAll(ctx context.Context, maxPOs int) (uuid.UUID, bool, error)
	RetryByID(ctx context.Context, poID uuid.UUID) (uuid.UUID, string, *string, error)
}

// Handler — корень всех action-методов фичи.
type Handler struct {
	svc ChannelService
}

// NewHandler создаёт Handler.
func NewHandler(svc ChannelService) *Handler {
	return &Handler{svc: svc}
}
