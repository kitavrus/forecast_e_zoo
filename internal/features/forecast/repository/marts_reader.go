package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/sqls/queries"
)

// ReadCalcInput — читает marts.mart_calculation_input.
//
// Если etlRunID == nil → читает все строки (последний committed может
// уже определять distribution в DB; в MVP marts всегда содержат текущий snapshot).
func (r *Repository) ReadCalcInput(
	ctx context.Context, etlRunID *uuid.UUID,
) ([]models.CalcInputRow, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("read_calc_input"), etlRunID)
	if err != nil {
		return nil, fmt.Errorf("forecast: read calc input: %w", err)
	}
	defer rows.Close()

	out := make([]models.CalcInputRow, 0, 1024) //nolint:mnd // pre-alloc
	for rows.Next() {
		c := models.CalcInputRow{}
		if scanErr := rows.Scan(
			&c.ProductID, &c.LocationID, &c.OnHand, &c.InTransit,
			&c.DailyDemand, &c.SupplierID, &c.LeadTimeDays, &c.SafetyStock,
			&c.MinQty, &c.MaxQty,
		); scanErr != nil {
			return nil, fmt.Errorf("forecast: read calc input scan: %w", scanErr)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forecast: read calc input rows.Err: %w", err)
	}
	return out, nil
}

// ReadDemandWindow — читает marts.mart_demand_history в окне [from, to].
func (r *Repository) ReadDemandWindow(
	ctx context.Context, from, to time.Time,
) ([]models.DemandPoint, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("read_demand_window"), from, to)
	if err != nil {
		return nil, fmt.Errorf("forecast: read demand window: %w", err)
	}
	defer rows.Close()

	out := make([]models.DemandPoint, 0, 4096) //nolint:mnd // pre-alloc
	for rows.Next() {
		d := models.DemandPoint{}
		if scanErr := rows.Scan(&d.ProductID, &d.LocationID, &d.AsOfDate, &d.QtySold); scanErr != nil {
			return nil, fmt.Errorf("forecast: read demand window scan: %w", scanErr)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forecast: read demand window rows.Err: %w", err)
	}
	return out, nil
}

// ReadSupplierScores — агрегаты scorecard для fallback lead_time / fill_rate.
func (r *Repository) ReadSupplierScores(
	ctx context.Context, from, to time.Time,
) ([]models.SupplierScore, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("read_supplier_scorecard"), from, to)
	if err != nil {
		return nil, fmt.Errorf("forecast: read scorecard: %w", err)
	}
	defer rows.Close()

	out := make([]models.SupplierScore, 0, 64) //nolint:mnd // pre-alloc
	for rows.Next() {
		s := models.SupplierScore{}
		if scanErr := rows.Scan(&s.SupplierID, &s.LeadTimeActualAvg, &s.FillRateAvg); scanErr != nil {
			return nil, fmt.Errorf("forecast: read scorecard scan: %w", scanErr)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forecast: read scorecard rows.Err: %w", err)
	}
	return out, nil
}
