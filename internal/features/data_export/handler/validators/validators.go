// Package validators содержит формальные валидаторы HTTP-запросов:
// - go-playground/validator/v10 c кастомными правилами.
// - Хелперы парсинга курсора, диапазона event_date, лимита.
package validators

import (
	"fmt"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// NewValidator возвращает настроенный *validator.Validate с кастомными правилами.
func NewValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// uuid — собственная регистрация (validator/v10 уже умеет, но переопределим формально для контроля).
	_ = v.RegisterValidation("uuid", func(fl validator.FieldLevel) bool {
		_, err := uuid.Parse(fl.Field().String())
		return err == nil
	})
	// iso_date — YYYY-MM-DD.
	_ = v.RegisterValidation("iso_date", func(fl validator.FieldLevel) bool {
		_, err := time.Parse("2006-01-02", fl.Field().String())
		return err == nil
	})
	// cursor_b64 — пустая строка ИЛИ декодируется через models.Cursor.
	_ = v.RegisterValidation("cursor_b64", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		if s == "" {
			return true
		}
		var c models.Cursor
		return c.Decode(s) == nil
	})
	return v
}

// ParseCursor декодирует cursor из строки. Пустая строка → пустой Cursor (валиден).
// Любая ошибка декодирования → errorspkg.ErrInvalidCursor (с обёрнутой причиной).
func ParseCursor(s string) (models.Cursor, error) {
	var c models.Cursor
	if err := c.Decode(s); err != nil {
		return models.Cursor{}, errorspkg.ErrInvalidCursor.Wrap(err)
	}
	return c, nil
}

// ParseEventDateRange парсит обе даты (ISO YYYY-MM-DD) и проверяет from <= to.
// Любая проблема → errorspkg.ErrInvalidQuery.
func ParseEventDateRange(from, to string) (time.Time, time.Time, error) {
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, errorspkg.ErrInvalidQuery.WithMessage(
			fmt.Sprintf("event_date_from invalid: %v", err))
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, errorspkg.ErrInvalidQuery.WithMessage(
			fmt.Sprintf("event_date_to invalid: %v", err))
	}
	if f.After(t) {
		return time.Time{}, time.Time{}, errorspkg.ErrInvalidQuery.WithMessage(
			"event_date_from must be <= event_date_to")
	}
	return f, t, nil
}

// ParseLimit парсит limit; пустое — def, отрицательное/нечисло — ErrBadRequest, > max — clamp.
// Возвращает clamp-значение в пределах [1, max].
func ParseLimit(s string, def, maxVal int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errorspkg.ErrBadRequest.WithMessage(fmt.Sprintf("limit must be integer: %v", err))
	}
	if n <= 0 {
		return 0, errorspkg.ErrBadRequest.WithMessage("limit must be > 0")
	}
	if n > maxVal {
		n = maxVal
	}
	return n, nil
}
