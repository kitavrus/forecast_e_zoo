package errorspkg

import "net/http"

// --- Sentinel-ошибки Модуля 7 (channel-routing, CR-*) ---
//
// См. docs/features/channel-routing/design.md §5.

var (
	// ErrSendAttemptNotFound — 404, GET send-attempts/{id} не найден.
	ErrSendAttemptNotFound = &Error{
		Code:           "send_attempt_not_found",
		Message:        "send attempt not found",
		SupportMessage: SupportSendAttemptNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrChannelNotConfigured — 409, для supplier нет channel config.
	ErrChannelNotConfigured = &Error{
		Code:           "channel_not_configured",
		Message:        "channel is not configured for supplier",
		SupportMessage: SupportChannelNotConfigured,
		HTTP:           http.StatusConflict,
	}

	// ErrPONotReadyToSend — 409, PO не в статусе ready_to_send.
	ErrPONotReadyToSend = &Error{
		Code:           "po_not_ready_to_send",
		Message:        "purchase order is not in ready_to_send status",
		SupportMessage: SupportPONotReadyToSend,
		HTTP:           http.StatusConflict,
	}

	// ErrChannelUnavailable — 503, внешняя система недоступна (после retry).
	ErrChannelUnavailable = &Error{
		Code:           "channel_unavailable",
		Message:        "external channel is unavailable",
		SupportMessage: SupportChannelUnavailable,
		HTTP:           http.StatusServiceUnavailable,
	}

	// ErrChannelRoutingInProgress — 409, advisory lock busy при on-demand send.
	ErrChannelRoutingInProgress = &Error{
		Code:           "channel_routing_in_progress",
		Message:        "another channel routing run is in progress",
		SupportMessage: SupportChannelRoutingInProgress,
		HTTP:           http.StatusConflict,
	}

	// ErrChannelRoutingUnavailable — 503, scheduler не сконфигурирован.
	ErrChannelRoutingUnavailable = &Error{
		Code:           "channel_routing_unavailable",
		Message:        "channel routing is not configured",
		SupportMessage: SupportChannelRoutingUnavailable,
		HTTP:           http.StatusServiceUnavailable,
	}

	// ErrInvalidChannelType — 400, channel_type не из allowlist.
	ErrInvalidChannelType = &Error{
		Code:           "invalid_channel_type",
		Message:        "invalid channel type (allowed: erp_api|edi_x12|edi_edifact|1c_xml|crm)",
		SupportMessage: SupportInvalidChannelType,
		HTTP:           http.StatusBadRequest,
	}

	// ErrInvalidAuthMode — 400, auth_mode не из allowlist.
	ErrInvalidAuthMode = &Error{
		Code:           "invalid_auth_mode",
		Message:        "invalid auth mode (allowed: api_key|oauth2|mtls|none)",
		SupportMessage: SupportInvalidAuthMode,
		HTTP:           http.StatusBadRequest,
	}
)
