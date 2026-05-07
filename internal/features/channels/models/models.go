// Package models — domain types фичи channels (Module 7 channel-routing).
package models

import (
	"time"

	"github.com/google/uuid"
)

// SupplierChannelConfig — одна строка channels.supplier_channel_config.
type SupplierChannelConfig struct {
	SupplierID          string
	ChannelType         string
	EndpointURL         string
	AuthMode            string
	AuthCredentialsRef  *string
	TimeoutSec          int
	RetryMax            int
	IsActive            bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// SendAttempt — одна строка channels.send_attempts.
type SendAttempt struct {
	ID             uuid.UUID
	POID           uuid.UUID
	SupplierID     string
	ChannelType    string
	StartedAt      time.Time
	FinishedAt     *time.Time
	Status         string
	HTTPStatusCode *int
	RequestBody    *string
	ResponseBody   *string
	ErrorMessage   *string
	RetryCount     int
	ExternalRef    *string
}

// SendAttemptFilter — фильтр для list-запроса send-attempts.
type SendAttemptFilter struct {
	POID       *uuid.UUID
	SupplierID *string
	Status     *string
	From       *time.Time
	To         *time.Time
	Limit      int
	Cursor     string
}

// PurchaseOrderForSend — облегчённый view orders.purchase_orders для отправки.
type PurchaseOrderForSend struct {
	ID         uuid.UUID
	PONumber   string
	SupplierID string
	LocationID string
	TotalQty   float64
	Currency   string
	CreatedAt  time.Time
}

// UpsertChannelConfigInput — вход для PUT /v1/channels/configs/{supplier_id}.
type UpsertChannelConfigInput struct {
	SupplierID         string
	ChannelType        string
	EndpointURL        string
	AuthMode           string
	AuthCredentialsRef *string
	TimeoutSec         int
	RetryMax           int
	IsActive           bool
}

// SendRunResult — итог одного routing-run.
type SendRunResult struct {
	RunID         uuid.UUID
	POsProcessed  int
	POsSent       int
	POsFailed     int
	POsSkipped    int
}
