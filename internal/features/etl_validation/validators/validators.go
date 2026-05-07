// Package validators содержит лёгкие валидаторы формата запросов
// admin-endpoint-ов фичи etl_validation.
//
// Контракт: каждый Validate* возвращает либо nil, либо *errorspkg.Error
// (sentinel-обёртка с понятным supportMessage). Валидация бизнес-логики
// (например, "уже есть running run") живёт в service-слое, не здесь.
package validators

// Validator — публичный интерфейс, реализуется *Impl.
//
// Используется DI: handlers получают Validator через NewHandler(svc, validator).
type Validator interface {
	ValidatePostEtlRun(req PostEtlRunInput) error
	ValidateRetryEtlRun(runID string) error
	ValidateGetEtlRun(runID string) error
	ValidateListEtlRuns(query ListEtlRunsQuery) error
	ValidateMartRefresh(martName string) error
	ValidateListRejectLog(query ListRejectLogQuery) error
}

// Impl — конкретная реализация Validator.
type Impl struct{}

// New возвращает готовый Validator.
func New() *Impl { return &Impl{} }
