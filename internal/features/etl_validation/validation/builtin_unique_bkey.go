package validation

import "fmt"

// UniqueBusinessKeyRule обнаруживает дубликаты по составному business-ключу.
//
// Параметры YAML:
//   - keys: список колонок, образующих ключ.
//
// Семантика: первая встреча ключа — OK; вторая и далее — нарушение.
// Если значение хотя бы одной колонки пустое (NULL) — строка пропускается
// (валидация NULL-полей — отдельное правило null_required_field).
func UniqueBusinessKeyRule(rule Rule, ds *Dataset) []Violation {
	if len(rule.Keys) == 0 {
		return []Violation{{
			RuleName: rule.Name, Kind: rule.Kind, Entity: rule.Entity,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("rule %s misconfigured: keys required", rule.Name),
		}}
	}
	seen := make(map[string]struct{})
	var out []Violation
	for _, row := range ds.Rows(rule.Entity) {
		// Если хоть одна колонка-ключ NULL/пустая — не считаем дубликатом.
		hasAll := true
		for _, k := range rule.Keys {
			v, ok := asString(row, k)
			if !ok || v == "" {
				hasAll = false
				break
			}
		}
		if !hasAll {
			continue
		}
		key := compositeKey(row, rule.Keys)
		if _, dup := seen[key]; dup {
			out = append(out, Violation{
				RuleName:    rule.Name,
				Kind:        rule.Kind,
				Entity:      rule.Entity,
				BusinessKey: key,
				Severity:    rule.Severity,
				Message:     fmt.Sprintf("дубликат business-ключа %v=%s", rule.Keys, key),
			})
			continue
		}
		seen[key] = struct{}{}
	}
	return out
}
