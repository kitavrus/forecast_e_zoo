// Package constants — константы фичи orders (Order Builder, Module 6).
package constants

// PO statuses (orders.purchase_orders.status).
const (
	POStatusDraft          = "draft"
	POStatusReadyToSend    = "ready_to_send"
	POStatusSent           = "sent"
	POStatusConfirmedByERP = "confirmed_by_erp"
	POStatusReceived       = "received"
	POStatusCancelled      = "cancelled"
)

// POStatuses — все валидные значения для validators/swagger enums.
var POStatuses = []string{
	POStatusDraft,
	POStatusReadyToSend,
	POStatusSent,
	POStatusConfirmedByERP,
	POStatusReceived,
	POStatusCancelled,
}

// IsKnownPOStatus — true если значение допустимо.
func IsKnownPOStatus(s string) bool {
	for _, v := range POStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// IsCancellableStatus — статусы из которых разрешён переход в cancelled
// (т.е. PO ещё не получен/завершён бизнес-стороной).
func IsCancellableStatus(s string) bool {
	switch s {
	case POStatusDraft, POStatusReadyToSend, POStatusSent, POStatusConfirmedByERP:
		return true
	}
	return false
}

// IsRegeneratableStatus — статусы, при которых regenerate допустим
// (создание нового PO + cancel старого; sent/confirmed/received → запрещён).
func IsRegeneratableStatus(s string) bool {
	switch s {
	case POStatusDraft, POStatusReadyToSend:
		return true
	}
	return false
}

// Plan statuses, используемые builder'ом (subset из forecast.PlanStatus).
const (
	PlanStatusApproved  = "approved"
	PlanStatusConverted = "converted"
)

// Pricing source — источник unit_price для po_lines.
const (
	PricingSourceProduct         = "product"
	PricingSourceSupplierDefault = "supplier_default"
	PricingSourceMissing         = "missing"
)

// PricingSources — все валидные значения.
var PricingSources = []string{
	PricingSourceProduct,
	PricingSourceSupplierDefault,
	PricingSourceMissing,
}

// Defaults для resolver supplier/product.
const (
	DefaultCurrency    = "UAH"
	DefaultLeadTimeDay = 7
)

// AdvisoryLockKey — ключ pg_advisory_lock для order builder run.
// Значение взято из ASCII bytes "OBDRLDV" (Order Builder).
// Должно быть стабильным: переименование = пересечение с другими scheduler'ами.
const AdvisoryLockKey int64 = 0x4F4244524C4456 // "OBDRLDV"

// Параметры запросов и пагинации.
const (
	LimitDefault = 100
	LimitMax     = 1000

	// PONumberSeqWidth — ширина sequence-части в номере PO (zero-padded).
	PONumberSeqWidth = 6

	// MaxPlansPerBuildBatch — верхний лимит plans за один tick.
	MaxPlansPerBuildBatch = 500

	// MaxReasonLength — верхний лимит для cancel/regenerate reason.
	MaxReasonLength = 500
)
