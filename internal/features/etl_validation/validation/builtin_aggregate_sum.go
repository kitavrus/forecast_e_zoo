package validation

import (
	"fmt"
	"math"
)

// AggregateSumMatchesRule проверяет, что SUM(entity.sum_column)
// совпадает с SUM(ref_entity.ref_sum_column) с точностью 1e-6.
//
// Параметры YAML:
//   - sum_column: имя колонки в entity;
//   - ref_entity: справочная entity;
//   - ref_sum_column: имя колонки в ref_entity.
//
// Семантика: cross-entity invariant (например, "сумма проданного по чекам =
// сумме списаний из stock"). Если суммы расходятся — одно нарушение
// общего уровня (BusinessKey="").
func AggregateSumMatchesRule(rule Rule, ds *Dataset) []Violation {
	if rule.SumColumn == "" || rule.RefEntity == "" || rule.RefSum == "" {
		return []Violation{{
			RuleName: rule.Name, Kind: rule.Kind, Entity: rule.Entity,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("rule %s misconfigured: sum_column/ref_entity/ref_sum_column required", rule.Name),
		}}
	}
	left := sumColumn(ds.Rows(rule.Entity), rule.SumColumn)
	right := sumColumn(ds.Rows(rule.RefEntity), rule.RefSum)
	const eps = 1e-6
	if math.Abs(left-right) <= eps {
		return nil
	}
	return []Violation{{
		RuleName: rule.Name,
		Kind:     rule.Kind,
		Entity:   rule.Entity,
		Severity: rule.Severity,
		Message: fmt.Sprintf(
			"SUM(%s.%s)=%g не совпадает с SUM(%s.%s)=%g",
			rule.Entity, rule.SumColumn, left, rule.RefEntity, rule.RefSum, right,
		),
	}}
}

func sumColumn(rows []Row, col string) float64 {
	var s float64
	for _, r := range rows {
		if v, ok := asFloat(r, col); ok {
			s += v
		}
	}
	return s
}
