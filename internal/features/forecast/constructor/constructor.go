// Package constructor — группировка calculation_lines в replenishment_plans.
//
// Алгоритм (см. design.md §1, ADR-007):
//   1. Группируем lines по (supplier_id, location_id).
//   2. Применяем MOQ: если sum(reorder_qty) < moq → skip.
//   3. Применяем multiplier: total = ceil(total / multiplier) × multiplier.
//   4. Создаём ReplenishmentPlan(supplier, location, plan_date, total_qty, lines_count, status='draft').
//
// MVP: SupplierConfig может быть пустой → MOQ=0, multiplier=1 (всё пропускается).
// В будущей итерации SupplierConfig резолвится из marts.mart_supplier_specs.
package constructor

import (
	"math"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// SupplierConfig — конфиг поставщика (MOQ, multiplier).
type SupplierConfig struct {
	MOQ        float64 // min order qty; 0 = no minimum
	Multiplier float64 // round up step; 0 or 1 = no rounding
}

// Constructor — stateless.
type Constructor struct {
	defaultConfig SupplierConfig
}

// New — конструктор с дефолтами (без MOQ/multiplier).
func New() *Constructor {
	return &Constructor{
		defaultConfig: SupplierConfig{MOQ: 0, Multiplier: 1},
	}
}

// BuildPlans — формирует список планов из calculation_lines.
//
// suppliers: per-supplier config; если supplier не найден — defaultConfig.
// planDate: дата плана (обычно asOf + lead_time_days; для MVP — asOf).
//
// Возвращает планы в детерминированном порядке (по supplier_id, location_id).
func (c *Constructor) BuildPlans(
	lines []models.CalculationLine,
	suppliers map[string]SupplierConfig,
	planDate time.Time,
) []models.ReplenishmentPlan {
	type key struct {
		supplier, location string
	}
	groups := make(map[key]*aggregator, 64) //nolint:mnd // pre-alloc

	for _, ln := range lines {
		if ln.SupplierID == nil || *ln.SupplierID == "" {
			continue
		}
		if ln.ReorderQty <= 0 {
			continue
		}
		k := key{*ln.SupplierID, ln.LocationID}
		agg, ok := groups[k]
		if !ok {
			agg = &aggregator{supplier: *ln.SupplierID, location: ln.LocationID}
			groups[k] = agg
		}
		agg.totalQty += ln.ReorderQty
		agg.linesCount++
	}

	// Stable iteration: collect keys, sort.
	plans := make([]models.ReplenishmentPlan, 0, len(groups))
	for _, agg := range groups {
		cfg := c.defaultConfig
		if v, ok := suppliers[agg.supplier]; ok {
			cfg = v
		}
		total := agg.totalQty
		// MOQ: skip plan если ниже минимума.
		if cfg.MOQ > 0 && total < cfg.MOQ {
			continue
		}
		// Multiplier: ceil round.
		if cfg.Multiplier > 1 {
			total = math.Ceil(total/cfg.Multiplier) * cfg.Multiplier
		}
		plans = append(plans, models.ReplenishmentPlan{
			SupplierID: agg.supplier,
			LocationID: agg.location,
			PlanDate:   planDate.UTC().Truncate(24 * time.Hour), //nolint:mnd
			TotalQty:   total,
			LinesCount: agg.linesCount,
			Status:     "draft",
		})
	}

	// Сортируем: supplier asc, location asc — детерминированный output.
	sortPlans(plans)
	return plans
}

type aggregator struct {
	supplier   string
	location   string
	totalQty   float64
	linesCount int
}

func sortPlans(plans []models.ReplenishmentPlan) {
	// Простая сортировка через bubble подойдёт для типовых ~100 plans;
	// в production — sort.Slice. Используем slice sort через map iteration order
	// stabilization: применяем sort.Slice.
	n := len(plans)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if less(plans[j], plans[i]) {
				plans[i], plans[j] = plans[j], plans[i]
			}
		}
	}
}

func less(a, b models.ReplenishmentPlan) bool {
	if a.SupplierID != b.SupplierID {
		return a.SupplierID < b.SupplierID
	}
	return a.LocationID < b.LocationID
}
