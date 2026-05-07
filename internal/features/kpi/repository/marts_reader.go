package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/sqls/queries"
)

// ReadDemandHistoryAggregates возвращает агрегации mart_demand_history
// в окне [from, to] (inclusive) для расчёта OSA.
func (r *Repository) ReadDemandHistoryAggregates(
	ctx context.Context, from, to time.Time,
) ([]models.DemandHistoryAgg, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("select_demand_history_for_osa"), from, to)
	if err != nil {
		return nil, fmt.Errorf("kpi: demand history query: %w", err)
	}
	defer rows.Close()

	out := make([]models.DemandHistoryAgg, 0, 256) //nolint:mnd // эвристика pre-alloc
	for rows.Next() {
		var a models.DemandHistoryAgg
		if scanErr := rows.Scan(&a.ProductID, &a.LocationID, &a.DaysObserved, &a.DaysOOS); scanErr != nil {
			return nil, fmt.Errorf("kpi: demand history scan: %w", scanErr)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kpi: demand history rows.Err: %w", err)
	}
	return out, nil
}

// ReadCalculationInput возвращает все строки mart_calculation_input для Stock Days.
func (r *Repository) ReadCalculationInput(ctx context.Context) ([]models.CalcInputRow, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("select_calc_input_for_stock_days"))
	if err != nil {
		return nil, fmt.Errorf("kpi: calc input query: %w", err)
	}
	defer rows.Close()

	out := make([]models.CalcInputRow, 0, 256) //nolint:mnd // эвристика pre-alloc
	for rows.Next() {
		var c models.CalcInputRow
		if scanErr := rows.Scan(
			&c.ProductID, &c.LocationID, &c.OnHand, &c.InTransit, &c.DailyDemand, &c.SupplierID,
		); scanErr != nil {
			return nil, fmt.Errorf("kpi: calc input scan: %w", scanErr)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kpi: calc input rows.Err: %w", err)
	}
	return out, nil
}

// ReadSupplierScorecard читает mart_supplier_scorecard в окне недель для OTIF.
func (r *Repository) ReadSupplierScorecard(
	ctx context.Context, from, to time.Time,
) ([]models.SupplierScorecardRow, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("select_supplier_scorecard_for_otif"), from, to)
	if err != nil {
		return nil, fmt.Errorf("kpi: scorecard query: %w", err)
	}
	defer rows.Close()

	out := make([]models.SupplierScorecardRow, 0, 64) //nolint:mnd // эвристика pre-alloc
	for rows.Next() {
		var s models.SupplierScorecardRow
		if scanErr := rows.Scan(
			&s.SupplierID, &s.WeekStart, &s.LinesDelivered, &s.LinesLate, &s.QtyShortTotal, &s.FillRateAvg,
		); scanErr != nil {
			return nil, fmt.Errorf("kpi: scorecard scan: %w", scanErr)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kpi: scorecard rows.Err: %w", err)
	}
	return out, nil
}
