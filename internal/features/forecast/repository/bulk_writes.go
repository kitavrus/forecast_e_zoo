package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/sqls/queries"
)

// BulkInsertForecasts — pipeline-friendly INSERT через UNNEST.
//
// Параметр modelName проставляется на все строки (в MVP — единый forecaster).
func (r *Repository) BulkInsertForecasts(
	ctx context.Context, runID uuid.UUID, items []models.Forecast, modelName string, confidence float64,
) error {
	if len(items) == 0 {
		return nil
	}
	productIDs := make([]string, len(items))
	locationIDs := make([]string, len(items))
	dates := make([]string, len(items))
	qtys := make([]float64, len(items))
	lower := make([]float64, len(items))
	upper := make([]float64, len(items))
	for i, it := range items {
		productIDs[i] = it.ProductID
		locationIDs[i] = it.LocationID
		dates[i] = it.ForecastDate.Format("2006-01-02")
		qtys[i] = it.ForecastQty
		if it.LowerBound != nil {
			lower[i] = *it.LowerBound
		} else {
			lower[i] = it.ForecastQty
		}
		if it.UpperBound != nil {
			upper[i] = *it.UpperBound
		} else {
			upper[i] = it.ForecastQty
		}
	}
	_, err := r.pool.Exec(ctx, queries.MustGet("bulk_insert_forecasts"),
		runID, productIDs, locationIDs, dates, qtys, lower, upper, modelName, confidence)
	if err != nil {
		return fmt.Errorf("forecast: bulk insert forecasts: %w", err)
	}
	return nil
}

// BulkInsertCalculationLines — INSERT строк расчёта.
func (r *Repository) BulkInsertCalculationLines(
	ctx context.Context, runID uuid.UUID, items []models.CalculationLine,
) error {
	if len(items) == 0 {
		return nil
	}
	productIDs := make([]string, len(items))
	locationIDs := make([]string, len(items))
	supplierIDs := make([]string, len(items))
	currentStocks := make([]float64, len(items))
	inTransits := make([]float64, len(items))
	dailyDemands := make([]float64, len(items))
	leadTimes := make([]int, len(items))
	safetyStocks := make([]float64, len(items))
	reorderPoints := make([]float64, len(items))
	targetStocks := make([]float64, len(items))
	reorderQtys := make([]float64, len(items))
	for i, it := range items {
		productIDs[i] = it.ProductID
		locationIDs[i] = it.LocationID
		if it.SupplierID != nil {
			supplierIDs[i] = *it.SupplierID
		}
		currentStocks[i] = it.CurrentStock
		inTransits[i] = it.InTransit
		dailyDemands[i] = it.DailyDemand
		leadTimes[i] = it.LeadTimeDays
		safetyStocks[i] = it.SafetyStock
		reorderPoints[i] = it.ReorderPoint
		targetStocks[i] = it.TargetStock
		reorderQtys[i] = it.ReorderQty
	}
	_, err := r.pool.Exec(ctx, queries.MustGet("bulk_insert_calc_lines"),
		runID, productIDs, locationIDs, supplierIDs, currentStocks, inTransits,
		dailyDemands, leadTimes, safetyStocks, reorderPoints, targetStocks, reorderQtys)
	if err != nil {
		return fmt.Errorf("forecast: bulk insert calc lines: %w", err)
	}
	return nil
}

// BulkInsertPlans — INSERT replenishment_plans.
func (r *Repository) BulkInsertPlans(
	ctx context.Context, runID uuid.UUID, items []models.ReplenishmentPlan,
) error {
	if len(items) == 0 {
		return nil
	}
	supplierIDs := make([]string, len(items))
	locationIDs := make([]string, len(items))
	dates := make([]string, len(items))
	totals := make([]float64, len(items))
	counts := make([]int, len(items))
	for i, it := range items {
		supplierIDs[i] = it.SupplierID
		locationIDs[i] = it.LocationID
		dates[i] = it.PlanDate.Format("2006-01-02")
		totals[i] = it.TotalQty
		counts[i] = it.LinesCount
	}
	_, err := r.pool.Exec(ctx, queries.MustGet("bulk_insert_plans"),
		runID, supplierIDs, locationIDs, dates, totals, counts)
	if err != nil {
		return fmt.Errorf("forecast: bulk insert plans: %w", err)
	}
	return nil
}
