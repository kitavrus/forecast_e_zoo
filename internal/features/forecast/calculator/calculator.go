// Package calculator — расчёт reorder_point / safety_stock / target / qty.
//
// Формулы (см. design.md §1, ADR Calculator):
//   lead_time_demand = daily_demand × lead_time_days
//   safety_stock     = z × stddev × sqrt(lead_time_days)   // z = 1.96 (95% SL)
//   reorder_point    = safety_stock + lead_time_demand
//   cycle_stock      = daily_demand × cycle_days
//   target_stock     = reorder_point + cycle_stock
//   reorder_qty      = max(0, target_stock − current_stock − in_transit)
package calculator

import (
	"math"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/constants"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// Input — параметры одного расчёта.
type Input struct {
	ProductID    string
	LocationID   string
	SupplierID   *string
	CurrentStock float64
	InTransit    float64
	DailyDemand  float64
	LeadTimeDays int
	StdDev       float64 // если 0 → safety_stock derived from lead-time только
}

// Calculator — stateless.
type Calculator struct {
	z         float64
	cycleDays int
}

// New создаёт Calculator с дефолтами.
func New() *Calculator {
	return &Calculator{
		z:         constants.SafetyStockZ95,
		cycleDays: constants.CycleStockDays,
	}
}

// Compute — вычисляет одну строку CalculationLine для (product, location).
//
// При daily_demand=0 → reorder_qty = 0; safety/reorder остаются 0.
// При lead_time_days <= 0 → fallback constants.LeadTimeDefault.
func (c *Calculator) Compute(in Input) models.CalculationLine {
	leadTime := in.LeadTimeDays
	if leadTime <= 0 {
		leadTime = constants.LeadTimeDefault
	}

	leadTimeDemand := in.DailyDemand * float64(leadTime)
	safetyStock := c.z * in.StdDev * math.Sqrt(float64(leadTime))
	if safetyStock < 0 {
		safetyStock = 0
	}
	reorderPoint := safetyStock + leadTimeDemand
	cycleStock := in.DailyDemand * float64(c.cycleDays)
	targetStock := reorderPoint + cycleStock

	reorderQty := targetStock - in.CurrentStock - in.InTransit
	if reorderQty < 0 {
		reorderQty = 0
	}
	if in.DailyDemand <= 0 {
		// Без спроса не заказываем (даже если current_stock < target).
		reorderQty = 0
	}

	return models.CalculationLine{
		ProductID:    in.ProductID,
		LocationID:   in.LocationID,
		SupplierID:   in.SupplierID,
		CurrentStock: in.CurrentStock,
		InTransit:    in.InTransit,
		DailyDemand:  in.DailyDemand,
		LeadTimeDays: leadTime,
		SafetyStock:  safetyStock,
		ReorderPoint: reorderPoint,
		TargetStock:  targetStock,
		ReorderQty:   reorderQty,
	}
}

// ComputeBatch — обёртка над Compute для batch.
func (c *Calculator) ComputeBatch(inputs []Input) []models.CalculationLine {
	out := make([]models.CalculationLine, 0, len(inputs))
	for _, in := range inputs {
		out = append(out, c.Compute(in))
	}
	return out
}
