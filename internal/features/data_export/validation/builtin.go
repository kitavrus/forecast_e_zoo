package validation

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// defaultChecks — встроенные обработчики правил.
func defaultChecks() map[string]CheckFunc {
	return map[string]CheckFunc{
		"negative_qty":       negativeQty,
		"future_event_time":  futureEventTime,
		"negative_stock":     negativeStock,
		"duplicate_pk":       duplicatePK,
		"missing_required":   missingRequired,
		"numeric_range":      numericRange,
		"regex_match":        regexMatch,
	}
}

// asFloat безопасно конвертирует payload[field] в float64.
// Возвращает (val, true), если поле есть и числовое.
func asFloat(payload map[string]any, field string) (float64, bool) {
	if payload == nil || field == "" {
		return 0, false
	}
	v, ok := payload[field]
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

// asString безопасно конвертирует payload[field] в string.
func asString(payload map[string]any, field string) (string, bool) {
	if payload == nil || field == "" {
		return "", false
	}
	v, ok := payload[field]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// negativeQty: payload[field] >= 0 (qty < 0 → нарушение).
func negativeQty(rule Rule, payload map[string]any, _ *State) (bool, string) {
	field := rule.Field
	if field == "" {
		field = "qty"
	}
	val, ok := asFloat(payload, field)
	if !ok {
		return true, "" // missing → не наше дело (missing_required отдельно)
	}
	if val < 0 {
		return false, fmt.Sprintf("%s = %v < 0", field, val)
	}
	return true, ""
}

// futureEventTime: payload[field] (RFC3339 или time.Time) > now+15min → нарушение.
func futureEventTime(rule Rule, payload map[string]any, _ *State) (bool, string) {
	field := rule.Field
	if field == "" {
		field = "event_time"
	}
	v, ok := payload[field]
	if !ok || v == nil {
		return true, ""
	}
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	case string:
		parsed, err := time.Parse(time.RFC3339, x)
		if err != nil {
			return false, fmt.Sprintf("%s: bad RFC3339 (%v)", field, err)
		}
		t = parsed
	default:
		return false, fmt.Sprintf("%s: unsupported type %T", field, v)
	}
	threshold := time.Now().Add(15 * time.Minute)
	if t.After(threshold) {
		return false, fmt.Sprintf("%s in future: %s", field, t.Format(time.RFC3339))
	}
	return true, ""
}

// negativeStock: payload[qty_on_hand] >= 0.
func negativeStock(rule Rule, payload map[string]any, _ *State) (bool, string) {
	field := rule.Field
	if field == "" {
		field = "qty_on_hand"
	}
	val, ok := asFloat(payload, field)
	if !ok {
		return true, ""
	}
	if val < 0 {
		return false, fmt.Sprintf("%s = %v < 0", field, val)
	}
	return true, ""
}

// duplicatePK: фиксирует pk (по rule.Field) в State; повторное появление = violation.
// Если состояние не передано (state==nil), создаётся локальное (per-call) — для прямых тестов.
var fallbackStateOnce sync.Once
var fallbackState *State

func getFallbackState() *State {
	fallbackStateOnce.Do(func() { fallbackState = NewState("fallback") })
	return fallbackState
}

func duplicatePK(rule Rule, payload map[string]any, state *State) (bool, string) {
	pkField := rule.Field
	if pkField == "" {
		return true, ""
	}
	pkValue, ok := asString(payload, pkField)
	if !ok {
		// допускаем числовые pk
		if f, fok := asFloat(payload, pkField); fok {
			pkValue = fmt.Sprintf("%v", f)
		} else {
			return true, ""
		}
	}
	if state == nil {
		state = getFallbackState()
	}
	if state.markPK(rule.Entity, pkValue) {
		return true, ""
	}
	return false, fmt.Sprintf("duplicate pk %s=%s in entity %s", pkField, pkValue, rule.Entity)
}

// missingRequired: каждое из rule.Fields должно присутствовать (и не быть nil/"").
func missingRequired(rule Rule, payload map[string]any, _ *State) (bool, string) {
	missing := make([]string, 0)
	for _, f := range rule.Fields {
		v, ok := payload[f]
		if !ok || v == nil {
			missing = append(missing, f)
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return false, fmt.Sprintf("missing required: %s", strings.Join(missing, ","))
	}
	return true, ""
}

// numericRange: payload[field] ∈ [min, max].
func numericRange(rule Rule, payload map[string]any, _ *State) (bool, string) {
	val, ok := asFloat(payload, rule.Field)
	if !ok {
		return true, ""
	}
	if rule.Min != nil && val < *rule.Min {
		return false, fmt.Sprintf("%s = %v < min=%v", rule.Field, val, *rule.Min)
	}
	if rule.Max != nil && val > *rule.Max {
		return false, fmt.Sprintf("%s = %v > max=%v", rule.Field, val, *rule.Max)
	}
	return true, ""
}

// regexMatch: payload[field] подпадает под rule.Pattern.
func regexMatch(rule Rule, payload map[string]any, _ *State) (bool, string) {
	s, ok := asString(payload, rule.Field)
	if !ok {
		return true, ""
	}
	if rule.Pattern == "" {
		return true, ""
	}
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return false, fmt.Sprintf("rule %s: bad regex %q: %v", rule.ID, rule.Pattern, err)
	}
	if !re.MatchString(s) {
		return false, fmt.Sprintf("%s = %q does not match %s", rule.Field, s, rule.Pattern)
	}
	return true, ""
}
