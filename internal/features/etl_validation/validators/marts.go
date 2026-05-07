package validators

import (
	"slices"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ValidateMartRefresh проверяет имя mart для POST /admin/marts/{name}/refresh.
//
// Возвращает:
//   - ErrBadRequest, если имя пустое или не входит в список известных mart-ов;
//   - ErrMartRefreshNotSupported (EV-005), если mart известен, но ondemand
//     refresh для него не реализован (см. constants.MartRefreshable).
func (Impl) ValidateMartRefresh(name string) error {
	if name == "" {
		return errorspkg.NewBadRequest("Параметр name обязателен")
	}
	if !slices.Contains(constants.MartNames, name) {
		return errorspkg.NewBadRequest("Неизвестное имя mart")
	}
	if !slices.Contains(constants.MartRefreshable, name) {
		return errorspkg.ErrMartRefreshNotSupported
	}
	return nil
}
