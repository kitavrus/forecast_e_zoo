// Package calculators — чистые KPI-формулы (OSA, OTIF, Stock Days).
//
// Каждый калькулятор stateless: получает rows из marts.* + params калибровки,
// возвращает []ComputedSnapshot. Никакого I/O.
package calculators

// ComputedSnapshot — результат расчёта одной KPI-точки.
//
// Engine трансформирует это в repository.InsertSnapshotInput, добавляя
// AsOfDate, EtlRunID и CalibrationID из контекста run-а.
type ComputedSnapshot struct {
	KpiName   string
	ScopeType string
	ScopeID   *string
	Value     float64
}
