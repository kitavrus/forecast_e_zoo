package validation

import "fmt"

// FkExistsRule проверяет, что для каждой строки entity значение row[Column]
// присутствует в наборе значений ref_entity[ref_column].
//
// Параметры YAML:
//   - column: FK-колонка в entity;
//   - ref_entity: имя справочной entity;
//   - ref_column: PK-колонка в ref_entity.
//
// Семантика: NULL/пустые значения не нарушение (NULL-FK обрабатывается
// отдельным правилом null_required_field).
func FkExistsRule(rule Rule, ds *Dataset) []Violation {
	if rule.Column == "" || rule.RefEntity == "" || rule.RefColumn == "" {
		return []Violation{{
			RuleName: rule.Name, Kind: rule.Kind, Entity: rule.Entity,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("rule %s misconfigured: column/ref_entity/ref_column required", rule.Name),
		}}
	}
	refRows := ds.Rows(rule.RefEntity)
	refSet := make(map[string]struct{}, len(refRows))
	for _, r := range refRows {
		if v, ok := asString(r, rule.RefColumn); ok && v != "" {
			refSet[v] = struct{}{}
		}
	}

	var out []Violation
	for _, row := range ds.Rows(rule.Entity) {
		val, ok := asString(row, rule.Column)
		if !ok || val == "" {
			continue
		}
		if _, found := refSet[val]; !found {
			out = append(out, Violation{
				RuleName:    rule.Name,
				Kind:        rule.Kind,
				Entity:      rule.Entity,
				Field:       rule.Column,
				BusinessKey: businessKey(row),
				Severity:    rule.Severity,
				Message: fmt.Sprintf(
					"%s.%s=%q отсутствует в %s.%s",
					rule.Entity, rule.Column, val, rule.RefEntity, rule.RefColumn,
				),
			})
		}
	}
	return out
}
