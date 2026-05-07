package validation

import "fmt"

// NullRequiredFieldRule проверяет, что row[column] не NULL и не пустая строка.
//
// Параметры YAML:
//   - column: имя проверяемой колонки.
func NullRequiredFieldRule(rule Rule, ds *Dataset) []Violation {
	if rule.Column == "" {
		return []Violation{{
			RuleName: rule.Name, Kind: rule.Kind, Entity: rule.Entity,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("rule %s misconfigured: column required", rule.Name),
		}}
	}
	var out []Violation
	for _, row := range ds.Rows(rule.Entity) {
		v, ok := asString(row, rule.Column)
		if !ok || v == "" {
			out = append(out, Violation{
				RuleName:    rule.Name,
				Kind:        rule.Kind,
				Entity:      rule.Entity,
				Field:       rule.Column,
				BusinessKey: businessKey(row),
				Severity:    rule.Severity,
				Message:     fmt.Sprintf("%s.%s обязательно непустое", rule.Entity, rule.Column),
			})
		}
	}
	return out
}
