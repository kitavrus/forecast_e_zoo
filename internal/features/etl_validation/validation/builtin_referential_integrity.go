package validation

import "fmt"

// ReferentialIntegrityRule — обратная сторона FkExistsRule:
// для каждой строки parent (ref_entity) проверяет, что хотя бы одна
// строка child (entity) ссылается на неё через child[column] = parent[ref_column].
//
// Используется реже — например, для отлова сирот-родителей (поставщик
// без единого order_rule). Severity по умолчанию soft.
func ReferentialIntegrityRule(rule Rule, ds *Dataset) []Violation {
	if rule.Column == "" || rule.RefEntity == "" || rule.RefColumn == "" {
		return []Violation{{
			RuleName: rule.Name, Kind: rule.Kind, Entity: rule.Entity,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("rule %s misconfigured: column/ref_entity/ref_column required", rule.Name),
		}}
	}
	childRows := ds.Rows(rule.Entity)
	usedSet := make(map[string]struct{}, len(childRows))
	for _, r := range childRows {
		if v, ok := asString(r, rule.Column); ok && v != "" {
			usedSet[v] = struct{}{}
		}
	}

	var out []Violation
	for _, parent := range ds.Rows(rule.RefEntity) {
		val, ok := asString(parent, rule.RefColumn)
		if !ok || val == "" {
			continue
		}
		if _, used := usedSet[val]; !used {
			out = append(out, Violation{
				RuleName:    rule.Name,
				Kind:        rule.Kind,
				Entity:      rule.RefEntity,
				Field:       rule.RefColumn,
				BusinessKey: businessKey(parent),
				Severity:    rule.Severity,
				Message: fmt.Sprintf(
					"%s.%s=%q не используется ни одной строкой в %s.%s",
					rule.RefEntity, rule.RefColumn, val, rule.Entity, rule.Column,
				),
			})
		}
	}
	return out
}
