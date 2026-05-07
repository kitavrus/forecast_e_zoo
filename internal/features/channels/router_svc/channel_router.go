// Package router_svc — ChannelRouter orchestration: resolve config + dispatch to sender + persist.
//
// Flow одного PO:
//  1. resolve channel config (per supplier_id, is_active=true)
//  2. idempotency check: уже есть success attempt?
//  3. begin tx + InsertSendAttempt (status=pending)
//  4. select sender by channel_type from registry
//  5. sender.Send(ctx, in, cfg)
//  6. FinishSendAttempt + (success → MarkPOSentTx) в той же tx
//  7. commit; metrics; logs
package router_svc //nolint:revive,stylecheck // имя пакета router_svc интуитивнее, чем routing/router

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/sender"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Repo — узкий interface (DI seam) для ChannelRouter.
type Repo interface {
	SelectReadyToSendPOsTx(ctx context.Context, tx pgx.Tx, limit int) ([]models.PurchaseOrderForSend, error)
	GetPOByIDForSend(ctx context.Context, id uuid.UUID) (models.PurchaseOrderForSend, string, error)
	GetSupplierChannelConfig(ctx context.Context, supplierID string) (models.SupplierChannelConfig, error)
	InsertSendAttempt(ctx context.Context, tx pgx.Tx,
		poID uuid.UUID, supplierID, channelType, status string, retryCount int,
	) (uuid.UUID, time.Time, error)
	FinishSendAttempt(ctx context.Context, tx pgx.Tx,
		attemptID uuid.UUID, startedAt time.Time,
		status string, httpStatus *int,
		requestBody, responseBody, errorMessage *string,
		retryCount int, externalRef *string,
	) error
	MarkPOSentTx(ctx context.Context, tx pgx.Tx, poID uuid.UUID, channelType string) error
	FindExistingSuccessAttempt(ctx context.Context, poID uuid.UUID) (uuid.UUID, *string, bool, error)
}

// SenderRegistry — узкий interface для select sender.
type SenderRegistry interface {
	Get(channelType string) (sender.ChannelSender, error)
}

// Metrics — опциональные callbacks для Prometheus.
type Metrics struct {
	SendTotal      func(channel, status string)
	SendDuration   func(channel string, seconds float64)
	RetryCount     func(channel string, retries int)
	IdempotentHit  func(channel string)
}

// ChannelRouter — orchestrator routing-runs.
type ChannelRouter struct {
	repo     Repo
	pool     *pgxpool.Pool
	registry SenderRegistry
	logger   *slog.Logger
	metrics  Metrics
}

// New создаёт ChannelRouter.
func New(repo Repo, pool *pgxpool.Pool, reg SenderRegistry, logger *slog.Logger, m Metrics) *ChannelRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChannelRouter{
		repo:     repo,
		pool:     pool,
		registry: reg,
		logger:   logger,
		metrics:  m,
	}
}

// SendAll — обрабатывает batch: SELECT FOR UPDATE SKIP LOCKED все ready_to_send PO,
// для каждого вызывает sendOne в отдельной tx.
//
// Используется scheduler-ом и admin POST /v1/channels/send.
func (r *ChannelRouter) SendAll(ctx context.Context, maxPOs int) (models.SendRunResult, error) {
	if maxPOs <= 0 {
		maxPOs = constants.MaxPosPerRunDefault
	}
	runID := uuid.New()
	res := models.SendRunResult{RunID: runID}

	// 1. Select POs in tx (FOR UPDATE SKIP LOCKED).
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return res, fmt.Errorf("router_svc: BeginTx: %w", err)
	}
	pos, err := r.repo.SelectReadyToSendPOsTx(ctx, tx, maxPOs)
	if err != nil {
		_ = tx.Rollback(ctx)
		return res, fmt.Errorf("router_svc: select ready_to_send: %w", err)
	}
	if rbErr := tx.Commit(ctx); rbErr != nil { // мы только читали — закрываем
		return res, fmt.Errorf("router_svc: commit select tx: %w", rbErr)
	}

	r.logger.InfoContext(ctx, "channel router: run started",
		slog.String("run_id", runID.String()),
		slog.Int("pos_to_process", len(pos)),
	)

	// 2. Process each PO in its own tx (изоляция фейла одного PO).
	for i := range pos {
		outcome := r.sendOne(ctx, pos[i], 0)
		switch outcome {
		case constants.SendAttemptStatusSuccess:
			res.POsSent++
		case constants.SendAttemptStatusSkipped:
			res.POsSkipped++
		default:
			res.POsFailed++
		}
	}
	res.POsProcessed = len(pos)

	r.logger.InfoContext(ctx, "channel router: run finished",
		slog.String("run_id", runID.String()),
		slog.Int("processed", res.POsProcessed),
		slog.Int("sent", res.POsSent),
		slog.Int("failed", res.POsFailed),
		slog.Int("skipped", res.POsSkipped),
	)
	return res, nil
}

// SendByID — retry конкретного PO (admin POST /:po_id/retry).
//
// Контракт:
//   - PO нет → ErrPurchaseOrderNotFound
//   - status != ready_to_send И уже был success attempt → возвращает success info (idempotent).
//   - status != ready_to_send И НЕ было success → ErrPONotReadyToSend
//   - всё ок → выполняем sendOne, возвращаем итоговый attemptID, status, externalRef
func (r *ChannelRouter) SendByID(ctx context.Context, poID uuid.UUID) (uuid.UUID, string, *string, error) {
	po, status, err := r.repo.GetPOByIDForSend(ctx, poID)
	if err != nil {
		return uuid.Nil, "", nil, fmt.Errorf("router_svc: get po: %w", err)
	}
	// Idempotency: если уже отправлено успешно — возвращаем success.
	existingID, externalRef, found, fErr := r.repo.FindExistingSuccessAttempt(ctx, poID)
	if fErr != nil {
		return uuid.Nil, "", nil, fmt.Errorf("router_svc: idempotency lookup: %w", fErr)
	}
	if found {
		if r.metrics.IdempotentHit != nil {
			cfg, _ := r.repo.GetSupplierChannelConfig(ctx, po.SupplierID)
			r.metrics.IdempotentHit(cfg.ChannelType)
		}
		return existingID, constants.SendAttemptStatusSuccess, externalRef, nil
	}
	// PO не в ready_to_send и нет успешного attempt → ошибка.
	if status != constants.POStatusReadyToSend {
		return uuid.Nil, "", nil, errorspkg.ErrPONotReadyToSend
	}

	outcome := r.sendOne(ctx, po, 0)
	// Найдём последний attempt чтобы вернуть id+externalRef.
	id, ext, _, _ := r.repo.FindExistingSuccessAttempt(ctx, poID)
	return id, outcome, ext, nil
}

// sendOne — отправка одного PO в отдельной tx. Возвращает финальный status.
//
//nolint:funlen,cyclop,gocognit // централизованная транзакция; делить вредно
func (r *ChannelRouter) sendOne(ctx context.Context, po models.PurchaseOrderForSend, retry int) string {
	cfg, err := r.repo.GetSupplierChannelConfig(ctx, po.SupplierID)
	if err != nil {
		r.logger.ErrorContext(ctx, "channel router: no channel config",
			slog.String("po_id", po.ID.String()),
			slog.String("supplier_id", po.SupplierID),
			slog.Any("error", err))
		if r.metrics.SendTotal != nil {
			r.metrics.SendTotal("unknown", constants.SendAttemptStatusSkipped)
		}
		return constants.SendAttemptStatusSkipped
	}

	channel, err := r.registry.Get(cfg.ChannelType)
	if err != nil {
		r.logger.ErrorContext(ctx, "channel router: sender not registered",
			slog.String("channel", cfg.ChannelType), slog.Any("error", err))
		if r.metrics.SendTotal != nil {
			r.metrics.SendTotal(cfg.ChannelType, constants.SendAttemptStatusSkipped)
		}
		return constants.SendAttemptStatusSkipped
	}

	// Begin tx: insert attempt (pending) → call sender → finish → mark sent.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		r.logger.ErrorContext(ctx, "channel router: begin tx", slog.Any("error", err))
		return constants.SendAttemptStatusFailed
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	attemptID, startedAt, err := r.repo.InsertSendAttempt(
		ctx, tx, po.ID, po.SupplierID, cfg.ChannelType, constants.SendAttemptStatusPending, retry,
	)
	if err != nil {
		r.logger.ErrorContext(ctx, "channel router: insert attempt", slog.Any("error", err))
		return constants.SendAttemptStatusFailed
	}

	// Sender — с учётом per-config timeout.
	res, sendErr := channel.Send(ctx, sender.SendInput{PO: po}, cfg)

	// Persist outcome (всегда, даже при ошибке).
	finStatus := res.Status
	if finStatus == "" || finStatus == constants.SendAttemptStatusPending {
		finStatus = constants.SendAttemptStatusFailed
	}
	if finishErr := r.repo.FinishSendAttempt(
		ctx, tx, attemptID, startedAt,
		finStatus, res.HTTPStatusCode,
		res.RequestBody, res.ResponseBody, res.ErrorMessage,
		res.RetryCount, res.ExternalRef,
	); finishErr != nil {
		r.logger.ErrorContext(ctx, "channel router: finish attempt", slog.Any("error", finishErr))
		return constants.SendAttemptStatusFailed
	}

	if finStatus == constants.SendAttemptStatusSuccess {
		if markErr := r.repo.MarkPOSentTx(ctx, tx, po.ID, cfg.ChannelType); markErr != nil {
			if errors.Is(markErr, errorspkg.ErrPONotReadyToSend) {
				r.logger.WarnContext(ctx, "channel router: PO not in ready_to_send (race)",
					slog.String("po_id", po.ID.String()))
				return constants.SendAttemptStatusSkipped
			}
			r.logger.ErrorContext(ctx, "channel router: mark sent", slog.Any("error", markErr))
			return constants.SendAttemptStatusFailed
		}
	}

	if cmErr := tx.Commit(ctx); cmErr != nil {
		r.logger.ErrorContext(ctx, "channel router: commit", slog.Any("error", cmErr))
		return constants.SendAttemptStatusFailed
	}
	committed = true

	// Metrics.
	if r.metrics.SendTotal != nil {
		r.metrics.SendTotal(cfg.ChannelType, finStatus)
	}
	if r.metrics.SendDuration != nil && res.FinishedAt != nil {
		r.metrics.SendDuration(cfg.ChannelType, res.FinishedAt.Sub(res.StartedAt).Seconds())
	}
	if r.metrics.RetryCount != nil && res.RetryCount > 0 {
		r.metrics.RetryCount(cfg.ChannelType, res.RetryCount)
	}

	if sendErr != nil {
		r.logger.WarnContext(ctx, "channel router: send returned error",
			slog.String("po_id", po.ID.String()),
			slog.String("status", finStatus),
			slog.Any("error", sendErr),
		)
	}
	return finStatus
}
