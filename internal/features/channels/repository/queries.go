// Package repository — pgx-based queries фичи channels.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// SelectReadyToSendPOsTx — FOR UPDATE SKIP LOCKED выборка PO в статусе ready_to_send.
func (r *Repository) SelectReadyToSendPOsTx(
	ctx context.Context, tx pgx.Tx, limit int,
) ([]models.PurchaseOrderForSend, error) {
	rows, err := tx.Query(ctx, queries.MustGet("select_ready_to_send_pos"), limit)
	if err != nil {
		return nil, fmt.Errorf("repository: SelectReadyToSendPOsTx: %w", err)
	}
	defer rows.Close()
	out := make([]models.PurchaseOrderForSend, 0, limit)
	for rows.Next() {
		var p models.PurchaseOrderForSend
		if scanErr := rows.Scan(
			&p.ID, &p.PONumber, &p.SupplierID, &p.LocationID,
			&p.TotalQty, &p.Currency, &p.CreatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("repository: SelectReadyToSendPOsTx scan: %w", scanErr)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: SelectReadyToSendPOsTx rows.Err: %w", err)
	}
	return out, nil
}

// GetPOByIDForSend возвращает PO без lock (для retry path).
func (r *Repository) GetPOByIDForSend(
	ctx context.Context, id uuid.UUID,
) (models.PurchaseOrderForSend, string, error) {
	var p models.PurchaseOrderForSend
	var status string
	row := r.pool.QueryRow(ctx, queries.MustGet("select_po_by_id_for_send"), id)
	err := row.Scan(
		&p.ID, &p.PONumber, &p.SupplierID, &p.LocationID,
		&status, &p.TotalQty, &p.Currency, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, "", errorspkg.ErrPurchaseOrderNotFound
		}
		return p, "", fmt.Errorf("repository: GetPOByIDForSend: %w", err)
	}
	return p, status, nil
}

// GetSupplierChannelConfig возвращает active config или ErrChannelNotConfigured.
func (r *Repository) GetSupplierChannelConfig(
	ctx context.Context, supplierID string,
) (models.SupplierChannelConfig, error) {
	var c models.SupplierChannelConfig
	row := r.pool.QueryRow(ctx,
		queries.MustGet("select_supplier_channel_config"), supplierID)
	err := row.Scan(
		&c.SupplierID, &c.ChannelType, &c.EndpointURL,
		&c.AuthMode, &c.AuthCredentialsRef, &c.TimeoutSec, &c.RetryMax,
		&c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c, errorspkg.ErrChannelNotConfigured
		}
		return c, fmt.Errorf("repository: GetSupplierChannelConfig: %w", err)
	}
	return c, nil
}

// ListChannelConfigs — все конфиги (включая inactive) для admin.
func (r *Repository) ListChannelConfigs(
	ctx context.Context,
) ([]models.SupplierChannelConfig, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("list_channel_configs"))
	if err != nil {
		return nil, fmt.Errorf("repository: ListChannelConfigs: %w", err)
	}
	defer rows.Close()
	out := make([]models.SupplierChannelConfig, 0)
	for rows.Next() {
		var c models.SupplierChannelConfig
		if scanErr := rows.Scan(
			&c.SupplierID, &c.ChannelType, &c.EndpointURL,
			&c.AuthMode, &c.AuthCredentialsRef, &c.TimeoutSec, &c.RetryMax,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("repository: ListChannelConfigs scan: %w", scanErr)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: ListChannelConfigs rows.Err: %w", err)
	}
	return out, nil
}

// UpsertChannelConfig — INSERT ON CONFLICT supplier_id.
func (r *Repository) UpsertChannelConfig(
	ctx context.Context, in models.UpsertChannelConfigInput,
) (models.SupplierChannelConfig, error) {
	var out models.SupplierChannelConfig
	row := r.pool.QueryRow(ctx,
		queries.MustGet("upsert_supplier_channel_config"),
		in.SupplierID, in.ChannelType, in.EndpointURL, in.AuthMode,
		in.AuthCredentialsRef, in.TimeoutSec, in.RetryMax, in.IsActive,
	)
	err := row.Scan(
		&out.SupplierID, &out.ChannelType, &out.EndpointURL,
		&out.AuthMode, &out.AuthCredentialsRef, &out.TimeoutSec, &out.RetryMax,
		&out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return out, fmt.Errorf("repository: UpsertChannelConfig: %w", err)
	}
	return out, nil
}

// InsertSendAttempt создаёт запись с status='pending' и возвращает (id, started_at).
//
// Поддерживает работу как через pool (передать tx=nil), так и через tx (если не nil).
func (r *Repository) InsertSendAttempt(
	ctx context.Context, tx pgx.Tx,
	poID uuid.UUID, supplierID, channelType, status string, retryCount int,
) (uuid.UUID, time.Time, error) {
	var (
		id        uuid.UUID
		startedAt time.Time
	)
	row := r.qrow(tx, ctx, queries.MustGet("insert_send_attempt"),
		poID, supplierID, channelType, status, retryCount)
	if err := row.Scan(&id, &startedAt); err != nil {
		return id, startedAt, fmt.Errorf("repository: InsertSendAttempt: %w", err)
	}
	return id, startedAt, nil
}

// FinishSendAttempt — UPDATE финализирует attempt.
func (r *Repository) FinishSendAttempt(
	ctx context.Context, tx pgx.Tx,
	attemptID uuid.UUID, startedAt time.Time,
	status string, httpStatus *int,
	requestBody, responseBody, errorMessage *string,
	retryCount int, externalRef *string,
) error {
	_, err := r.qexec(tx, ctx,
		queries.MustGet("update_send_attempt_finish"),
		attemptID, startedAt,
		status, httpStatus, requestBody, responseBody,
		errorMessage, retryCount, externalRef,
	)
	if err != nil {
		return fmt.Errorf("repository: FinishSendAttempt: %w", err)
	}
	return nil
}

// MarkPOSentTx — UPDATE PO в статус sent (внутри транзакции с send_attempts).
func (r *Repository) MarkPOSentTx(
	ctx context.Context, tx pgx.Tx, poID uuid.UUID, channelType string,
) error {
	tag, err := tx.Exec(ctx,
		queries.MustGet("update_po_to_sent"), poID, channelType)
	if err != nil {
		return fmt.Errorf("repository: MarkPOSentTx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errorspkg.ErrPONotReadyToSend
	}
	return nil
}

// FindExistingSuccessAttempt — idempotency lookup.
//
// Возвращает (id, externalRef, true) если есть, иначе (zero, nil, false).
func (r *Repository) FindExistingSuccessAttempt(
	ctx context.Context, poID uuid.UUID,
) (uuid.UUID, *string, bool, error) {
	var (
		id          uuid.UUID
		externalRef *string
		startedAt   time.Time
	)
	row := r.pool.QueryRow(ctx,
		queries.MustGet("select_existing_success_attempt"), poID)
	err := row.Scan(&id, &externalRef, &startedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil, false, nil
		}
		return uuid.Nil, nil, false, fmt.Errorf(
			"repository: FindExistingSuccessAttempt: %w", err)
	}
	return id, externalRef, true, nil
}

// ListSendAttempts с фильтрами + cursor pagination.
func (r *Repository) ListSendAttempts(
	ctx context.Context, f models.SendAttemptFilter,
) ([]models.SendAttempt, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50 //nolint:mnd // default
	}
	var cursorID *uuid.UUID
	if f.Cursor != "" {
		id, err := uuid.Parse(f.Cursor)
		if err != nil {
			return nil, "", errorspkg.ErrBadRequest.WithMessage("invalid cursor")
		}
		cursorID = &id
	}
	rows, err := r.pool.Query(ctx,
		queries.MustGet("select_send_attempts"),
		f.POID, f.SupplierID, f.Status, f.From, f.To, cursorID, limit,
	)
	if err != nil {
		return nil, "", fmt.Errorf("repository: ListSendAttempts: %w", err)
	}
	defer rows.Close()
	out := make([]models.SendAttempt, 0, limit)
	for rows.Next() {
		var a models.SendAttempt
		if scanErr := rows.Scan(
			&a.ID, &a.POID, &a.SupplierID, &a.ChannelType,
			&a.StartedAt, &a.FinishedAt, &a.Status,
			&a.HTTPStatusCode, &a.ErrorMessage,
			&a.RetryCount, &a.ExternalRef,
		); scanErr != nil {
			return nil, "", fmt.Errorf("repository: ListSendAttempts scan: %w", scanErr)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("repository: ListSendAttempts rows.Err: %w", err)
	}
	var nextCursor string
	if len(out) == limit {
		nextCursor = out[len(out)-1].ID.String()
	}
	return out, nextCursor, nil
}

// GetSendAttemptByID — детальный (с request/response bodies).
func (r *Repository) GetSendAttemptByID(
	ctx context.Context, id uuid.UUID,
) (models.SendAttempt, error) {
	var a models.SendAttempt
	row := r.pool.QueryRow(ctx,
		queries.MustGet("select_send_attempt_by_id"), id)
	err := row.Scan(
		&a.ID, &a.POID, &a.SupplierID, &a.ChannelType,
		&a.StartedAt, &a.FinishedAt, &a.Status,
		&a.HTTPStatusCode, &a.RequestBody, &a.ResponseBody,
		&a.ErrorMessage, &a.RetryCount, &a.ExternalRef,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return a, errorspkg.ErrSendAttemptNotFound
		}
		return a, fmt.Errorf("repository: GetSendAttemptByID: %w", err)
	}
	return a, nil
}

// --- helpers (tx-vs-pool dispatcher) ---

func (r *Repository) qrow(tx pgx.Tx, ctx context.Context, sql string, args ...any) pgx.Row {
	if tx != nil {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}

func (r *Repository) qexec(
	tx pgx.Tx, ctx context.Context, sql string, args ...any,
) (pgconn.CommandTag, error) {
	if tx != nil {
		tag, err := tx.Exec(ctx, sql, args...)
		if err != nil {
			return tag, fmt.Errorf("tx.Exec: %w", err)
		}
		return tag, nil
	}
	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return tag, fmt.Errorf("pool.Exec: %w", err)
	}
	return tag, nil
}
