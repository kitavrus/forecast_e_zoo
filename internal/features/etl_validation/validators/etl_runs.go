package validators

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// PostEtlRunInput — лёгкая структура для будущей расширяемости тела POST.
//
// Сейчас тело пустое, но валидатор сохраняет точку расширения.
type PostEtlRunInput struct{}

// ListEtlRunsQuery — query-параметры GET /admin/etl-runs.
type ListEtlRunsQuery struct {
	Status string // "" допустимо
	Kind   string // "" допустимо
	Cursor string // RFC3339, "" допустимо
	Limit  int    // 0 → допустимо (defaultsLimitDefault)
}

// ValidatePostEtlRun — пустой body валидируется как OK.
func (Impl) ValidatePostEtlRun(_ PostEtlRunInput) error { return nil }

// ValidateRetryEtlRun — id обязан быть валидным UUID.
func (Impl) ValidateRetryEtlRun(runID string) error { return validateUUID(runID, "id") }

// ValidateGetEtlRun — id обязан быть валидным UUID.
func (Impl) ValidateGetEtlRun(runID string) error { return validateUUID(runID, "id") }

// ValidateListEtlRuns проверяет enum-поля и диапазон limit.
func (Impl) ValidateListEtlRuns(q ListEtlRunsQuery) error {
	if q.Status != "" && !slices.Contains(constants.EtlRunStatuses, q.Status) {
		return errorspkg.NewBadRequest("Некорректное значение query-параметра status")
	}
	if q.Kind != "" && !slices.Contains(constants.EtlRunKinds, q.Kind) {
		return errorspkg.NewBadRequest("Некорректное значение query-параметра kind")
	}
	if q.Cursor != "" {
		if _, err := time.Parse(time.RFC3339, q.Cursor); err != nil {
			return errorspkg.NewBadRequest("Некорректный курсор: ожидается RFC3339-timestamp")
		}
	}
	if q.Limit != 0 && (q.Limit < constants.EtlRunsListLimitMin || q.Limit > constants.EtlRunsListLimitMax) {
		return errorspkg.NewBadRequest("limit должен быть в диапазоне [1; 100]")
	}
	return nil
}

func validateUUID(s, field string) error {
	if s == "" {
		return errorspkg.NewBadRequest("Параметр " + field + " обязателен")
	}
	if _, err := uuid.Parse(s); err != nil {
		return errorspkg.NewBadRequest("Параметр " + field + " должен быть UUID")
	}
	return nil
}
