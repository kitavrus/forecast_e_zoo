package validation

import (
	"fmt"
	"strings"
)

// asString — безопасный извлекатель строкового значения из row[col].
// Поддерживает string и []byte; для остальных типов — fmt.Sprint.
func asString(row Row, col string) (string, bool) {
	if col == "" {
		return "", false
	}
	v, ok := row[col]
	if !ok || v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		return string(x), true
	}
	return fmt.Sprint(v), true
}

// asFloat — безопасный извлекатель числового значения из row[col].
func asFloat(row Row, col string) (float64, bool) {
	if col == "" {
		return 0, false
	}
	v, ok := row[col]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

// compositeKey строит составной ключ row[col1]+"|"+row[col2]+...
func compositeKey(row Row, cols []string) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		s, _ := asString(row, c)
		parts = append(parts, s)
	}
	return strings.Join(parts, "|")
}

// businessKey формирует читаемое представление строки для reject_log.business_key.
//
// Если row содержит "id" — берём его; иначе склеиваем все ключи в алфавитном порядке.
func businessKey(row Row) string {
	if id, ok := asString(row, "id"); ok {
		return id
	}
	return fmt.Sprintf("%v", row)
}
