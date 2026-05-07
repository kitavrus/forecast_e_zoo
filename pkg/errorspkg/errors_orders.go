package errorspkg

import "net/http"

// --- Sentinel-ошибки Модуля 6 (order-builder, OB-*) ---
//
// См. docs/features/order-builder/design.md §4.

var (
	// ErrPurchaseOrderNotFound — 404, GET PO/{id} не найден.
	ErrPurchaseOrderNotFound = &Error{
		Code:           "purchase_order_not_found",
		Message:        "purchase order not found",
		SupportMessage: SupportPurchaseOrderNotFound,
		HTTP:           http.StatusNotFound,
	}

	// ErrPlanAlreadyConverted — 409, попытка сбилдить уже сконвертированный plan.
	ErrPlanAlreadyConverted = &Error{
		Code:           "plan_already_converted",
		Message:        "plan already converted to PO",
		SupportMessage: SupportPlanAlreadyConverted,
		HTTP:           http.StatusConflict,
	}

	// ErrPlanNotApproved — 409, plan не в статусе approved.
	ErrPlanNotApproved = &Error{
		Code:           "plan_not_approved",
		Message:        "plan must be approved to build PO",
		SupportMessage: SupportPlanNotApproved,
		HTTP:           http.StatusConflict,
	}

	// ErrPONotCancellable — 409, PO в финальном/некорректном статусе для отмены.
	ErrPONotCancellable = &Error{
		Code:           "po_not_cancellable",
		Message:        "PO cannot be cancelled in current status",
		SupportMessage: SupportPONotCancellable,
		HTTP:           http.StatusConflict,
	}

	// ErrPOAlreadySent — 409, regenerate невозможен после sent.
	ErrPOAlreadySent = &Error{
		Code:           "po_already_sent",
		Message:        "PO already sent and cannot be regenerated",
		SupportMessage: SupportPOAlreadySent,
		HTTP:           http.StatusConflict,
	}

	// ErrInvalidPOStatus — 400, query ?status= не из allowlist.
	ErrInvalidPOStatus = &Error{
		Code:           "invalid_po_status",
		Message:        "invalid PO status (allowed: draft|ready_to_send|sent|confirmed_by_erp|received|cancelled)",
		SupportMessage: SupportInvalidPOStatus,
		HTTP:           http.StatusBadRequest,
	}

	// ErrOrderBuilderUnavailable — 503, scheduler/builder не сконфигурированы.
	ErrOrderBuilderUnavailable = &Error{
		Code:           "order_builder_unavailable",
		Message:        "order builder is not configured",
		SupportMessage: SupportOrderBuilderUnavailable,
		HTTP:           http.StatusServiceUnavailable,
	}

	// ErrOrderBuilderInProgress — 409, advisory_lock busy при on-demand build.
	ErrOrderBuilderInProgress = &Error{
		Code:           "order_builder_in_progress",
		Message:        "another build run is in progress",
		SupportMessage: SupportOrderBuilderInProgress,
		HTTP:           http.StatusConflict,
	}
)
