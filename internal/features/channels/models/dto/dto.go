// Package dto — request/response DTO фичи channels (Module 7).
package dto

import (
	"time"

	"github.com/google/uuid"
)

// SendAttemptItem — DTO одной строки send_attempts.
type SendAttemptItem struct {
	ID             uuid.UUID  `json:"id"`
	POID           uuid.UUID  `json:"po_id"`
	SupplierID     string     `json:"supplier_id"`
	ChannelType    string     `json:"channel_type" enums:"erp_api,edi_x12,edi_edifact,1c_xml,crm"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	Status         string     `json:"status" enums:"pending,success,failed,skipped"`
	HTTPStatusCode *int       `json:"http_status_code,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	RetryCount     int        `json:"retry_count"`
	ExternalRef    *string    `json:"external_ref,omitempty"`
}

// SendAttemptDetail — расширенный DTO с request/response bodies.
type SendAttemptDetail struct {
	SendAttemptItem
	RequestBody  *string `json:"request_body,omitempty"`
	ResponseBody *string `json:"response_body,omitempty"`
}

// ListSendAttemptsResponse — ответ GET /v1/channels/send-attempts.
type ListSendAttemptsResponse struct {
	Items      []SendAttemptItem `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// SendAttemptDetailResponse — ответ GET /v1/channels/send-attempts/:id.
type SendAttemptDetailResponse struct {
	Attempt SendAttemptDetail `json:"attempt"`
}

// TriggerSendRequest — body POST /v1/channels/send.
type TriggerSendRequest struct {
	MaxPos int `json:"max_pos,omitempty"`
}

// TriggerSendResponse — ответ POST /v1/channels/send (202 Accepted).
type TriggerSendResponse struct {
	RunID   uuid.UUID `json:"run_id"`
	Started bool      `json:"started"`
}

// RetryResponse — ответ POST /v1/channels/send/:po_id/retry.
type RetryResponse struct {
	AttemptID   uuid.UUID `json:"attempt_id"`
	Status      string    `json:"status" enums:"pending,success,failed,skipped"`
	ExternalRef *string   `json:"external_ref,omitempty"`
}

// ChannelConfigItem — DTO одной строки supplier_channel_config.
type ChannelConfigItem struct {
	SupplierID         string    `json:"supplier_id"`
	ChannelType        string    `json:"channel_type" enums:"erp_api,edi_x12,edi_edifact,1c_xml,crm"`
	EndpointURL        string    `json:"endpoint_url"`
	AuthMode           string    `json:"auth_mode" enums:"api_key,oauth2,mtls,none"`
	AuthCredentialsRef *string   `json:"auth_credentials_ref,omitempty"`
	TimeoutSec         int       `json:"timeout_sec"`
	RetryMax           int       `json:"retry_max"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ListChannelConfigsResponse — ответ GET /v1/channels/configs.
type ListChannelConfigsResponse struct {
	Items []ChannelConfigItem `json:"items"`
}

// UpsertChannelConfigRequest — body PUT /v1/channels/configs/:supplier_id.
type UpsertChannelConfigRequest struct {
	ChannelType        string  `json:"channel_type" validate:"required" enums:"erp_api,edi_x12,edi_edifact,1c_xml,crm"`
	EndpointURL        string  `json:"endpoint_url" validate:"required"`
	AuthMode           string  `json:"auth_mode" validate:"required" enums:"api_key,oauth2,mtls,none"`
	AuthCredentialsRef *string `json:"auth_credentials_ref,omitempty"`
	TimeoutSec         int     `json:"timeout_sec,omitempty"`
	RetryMax           int     `json:"retry_max,omitempty"`
	IsActive           *bool   `json:"is_active,omitempty"`
}
