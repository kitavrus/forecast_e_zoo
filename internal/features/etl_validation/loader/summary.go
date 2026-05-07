// Package loader выполняет атомарный snapshot-flip ETL run в одной транзакции:
// последовательно строит все mart-ы (через transformer.Builder) и помечает
// marts.etl_runs как 'committed' при успехе или откатывается при ошибке.
package loader

import (
	"encoding/json"
	"fmt"
)

// BuildSummary — отчёт по построенным mart-ам.
//
// Ключ — имя mart, значение — количество inserted/updated rows.
type BuildSummary map[string]int64

// NewBuildSummary возвращает пустую карту.
func NewBuildSummary() BuildSummary { return make(BuildSummary) }

// Add регистрирует rows для martName (idempotent — последний вызов перетирает).
func (s BuildSummary) Add(martName string, rows int64) {
	if martName == "" {
		return
	}
	s[martName] = rows
}

// Total — сумма rows по всем mart-ам.
func (s BuildSummary) Total() int64 {
	var total int64
	for _, n := range s {
		total += n
	}
	return total
}

// MarshalJSONB сериализует summary в формате `{mart_name: {rows: N}}`,
// ожидаемый etl_runs.marts_summary (см. design-go-layers.md).
func (s BuildSummary) MarshalJSONB() ([]byte, error) {
	out := make(map[string]map[string]int64, len(s))
	for k, v := range s {
		out[k] = map[string]int64{"rows": v}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("loader: marshal summary: %w", err)
	}
	return raw, nil
}
