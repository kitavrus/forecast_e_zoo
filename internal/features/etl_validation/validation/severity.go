// Package validation содержит cross-entity validation engine и набор
// builtin-правил (fk_exists, unique_business_key, aggregate_sum_matches,
// referential_integrity, null_required_field) для ETL-pipeline фичи
// etl_validation.
//
// В отличие от row-level engine из internal/features/data_export/validation,
// здесь правила работают над всем Dataset — staging-таблицами целиком.
package validation

// Severity — severity-уровень правила (совпадает с reject_log.severity).
type Severity string

const (
	// SeverityCritical — нарушение, после которого ETL run обязан перейти в failed.
	SeverityCritical Severity = "critical"
	// SeveritySoft — нарушение, регистрируемое в reject_log, но не прерывающее run.
	SeveritySoft Severity = "soft"
)

// IsValid — true, если sev одно из известных значений.
func (s Severity) IsValid() bool {
	return s == SeverityCritical || s == SeveritySoft
}
