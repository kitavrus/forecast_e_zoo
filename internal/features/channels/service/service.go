// Package service — бизнес-логика фичи channels (Module 7 channel-routing).
//
// Service оборачивает Repository (read) + ChannelRouter (orchestration) + Scheduler (trigger).
// Handlers вызывают только service-методы, не трогают репо/router напрямую.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Repo — узкий interface (DI seam).
type Repo interface {
	ListSendAttempts(ctx context.Context, f models.SendAttemptFilter) ([]models.SendAttempt, string, error)
	GetSendAttemptByID(ctx context.Context, id uuid.UUID) (models.SendAttempt, error)
	ListChannelConfigs(ctx context.Context) ([]models.SupplierChannelConfig, error)
	UpsertChannelConfig(ctx context.Context, in models.UpsertChannelConfigInput) (models.SupplierChannelConfig, error)
}

// Router — orchestrator (channels/router_svc).
type Router interface {
	SendByID(ctx context.Context, poID uuid.UUID) (uuid.UUID, string, *string, error)
}

// Trigger — scheduler API (TryTrigger).
type Trigger interface {
	TryTrigger(ctx context.Context, maxPOs int) (uuid.UUID, bool, error)
}

// Service — собирает читалки и оркестрационные вызовы.
type Service struct {
	repo    Repo
	router  Router
	trigger Trigger
}

// New создаёт Service.
//
// trigger может быть nil (если scheduler не сконфигурирован) — тогда TriggerSendAll
// вернёт ErrChannelRoutingUnavailable. router тоже может быть nil — тогда RetryByID
// вернёт ErrChannelRoutingUnavailable.
func New(repo Repo, router Router, trigger Trigger) *Service {
	return &Service{repo: repo, router: router, trigger: trigger}
}

// ListSendAttempts — для GET /v1/channels/send-attempts.
func (s *Service) ListSendAttempts(
	ctx context.Context, f models.SendAttemptFilter,
) ([]models.SendAttempt, string, error) {
	items, cursor, err := s.repo.ListSendAttempts(ctx, f)
	if err != nil {
		return nil, "", fmt.Errorf("service: ListSendAttempts: %w", err)
	}
	return items, cursor, nil
}

// GetSendAttempt — для GET /v1/channels/send-attempts/:id (с request/response logs).
func (s *Service) GetSendAttempt(
	ctx context.Context, id uuid.UUID,
) (models.SendAttempt, error) {
	a, err := s.repo.GetSendAttemptByID(ctx, id)
	if err != nil {
		return a, fmt.Errorf("service: GetSendAttempt: %w", err)
	}
	return a, nil
}

// ListConfigs — для GET /v1/channels/configs.
func (s *Service) ListConfigs(ctx context.Context) ([]models.SupplierChannelConfig, error) {
	items, err := s.repo.ListChannelConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: ListConfigs: %w", err)
	}
	return items, nil
}

// UpsertConfig — для PUT /v1/channels/configs/:supplier_id.
func (s *Service) UpsertConfig(
	ctx context.Context, in models.UpsertChannelConfigInput,
) (models.SupplierChannelConfig, error) {
	out, err := s.repo.UpsertChannelConfig(ctx, in)
	if err != nil {
		return out, fmt.Errorf("service: UpsertConfig: %w", err)
	}
	return out, nil
}

// TriggerSendAll — для POST /v1/channels/send (admin on-demand).
func (s *Service) TriggerSendAll(ctx context.Context, maxPOs int) (uuid.UUID, bool, error) {
	if s.trigger == nil {
		return uuid.Nil, false, errorspkg.ErrChannelRoutingUnavailable
	}
	runID, started, err := s.trigger.TryTrigger(ctx, maxPOs)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("service: TriggerSendAll: %w", err)
	}
	return runID, started, nil
}

// RetryByID — для POST /v1/channels/send/:po_id/retry (admin retry).
func (s *Service) RetryByID(
	ctx context.Context, poID uuid.UUID,
) (uuid.UUID, string, *string, error) {
	if s.router == nil {
		return uuid.Nil, "", nil, errorspkg.ErrChannelRoutingUnavailable
	}
	id, status, ext, err := s.router.SendByID(ctx, poID)
	if err != nil {
		// Доменные sentinel'ы пробрасываем без обёртки чтобы handler мог матчить.
		var domainErr *errorspkg.Error
		if errors.As(err, &domainErr) {
			return uuid.Nil, "", nil, err //nolint:wrapcheck // sentinel passthrough
		}
		return uuid.Nil, "", nil, fmt.Errorf("service: RetryByID: %w", err)
	}
	return id, status, ext, nil
}
