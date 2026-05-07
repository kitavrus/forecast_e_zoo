package validators

import (
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ListRejectLogQuery — query-параметры GET /admin/reject-log.
//
// Cursor — строковое представление BIGINT id (последний полученный).
type ListRejectLogQuery struct {
	EtlRunID string // "" допустимо
	Entity   string // "" допустимо
	Severity string // "" допустимо
	Cursor   string // "" допустимо, иначе int64 > 0
	Limit    int    // 0 → допустимо
}

// ValidateListRejectLog проверяет enum-поля, формат cursor и диапазон limit.
func (Impl) ValidateListRejectLog(q ListRejectLogQuery) error {
	if q.EtlRunID != "" {
		if _, err := uuid.Parse(q.EtlRunID); err != nil {
			return errorspkg.NewBadRequest("Некорректный etl_run_id: ожидается UUID")
		}
	}
	if q.Severity != "" && !slices.Contains(constants.RejectSeverities, q.Severity) {
		return errorspkg.NewBadRequest("Некорректное значение severity")
	}
	if q.Entity != "" && !slices.Contains(constants.AllowedEntities, q.Entity) {
		return errorspkg.NewBadRequest("Некорректное значение entity")
	}
	if q.Cursor != "" {
		v, err := strconv.ParseInt(q.Cursor, 10, 64)
		if err != nil || v <= 0 {
			return errorspkg.NewBadRequest("Некорректный курсор: ожидается положительное число")
		}
	}
	if q.Limit != 0 && (q.Limit < constants.RejectLogListLimitMin || q.Limit > constants.RejectLogListLimitMax) {
		return errorspkg.NewBadRequest("limit должен быть в диапазоне [1; 500]")
	}
	return nil
}
