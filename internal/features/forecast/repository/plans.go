package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/sqls/queries"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ListPlans — пагинация replenishment_plans.
func (r *Repository) ListPlans(
	ctx context.Context, f models.PlanFilter,
) ([]models.ReplenishmentPlan, string, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	var (
		sup      interface{} = nil
		loc      interface{} = nil
		date     interface{} = nil
		st       interface{} = nil
		cursorID interface{} = nil
	)
	if f.SupplierID != nil {
		sup = *f.SupplierID
	}
	if f.LocationID != nil {
		loc = *f.LocationID
	}
	if f.PlanDate != nil {
		date = *f.PlanDate
	}
	if f.Status != nil {
		st = *f.Status
	}
	if f.Cursor != "" {
		id, err := uuid.Parse(f.Cursor)
		if err != nil {
			return nil, "", errorspkg.ErrBadRequest.WithMessage("invalid cursor")
		}
		cursorID = id
	}

	rows, err := r.pool.Query(ctx, queries.MustGet("select_plans"),
		sup, loc, date, st, cursorID, limit)
	if err != nil {
		return nil, "", fmt.Errorf("forecast: list plans query: %w", err)
	}
	defer rows.Close()

	out := make([]models.ReplenishmentPlan, 0, limit)
	for rows.Next() {
		p := models.ReplenishmentPlan{}
		if scanErr := rows.Scan(
			&p.ID, &p.RunID, &p.SupplierID, &p.LocationID, &p.PlanDate,
			&p.TotalQty, &p.LinesCount, &p.Status, &p.ApprovedAt, &p.ApprovedBy, &p.CreatedAt,
		); scanErr != nil {
			return nil, "", fmt.Errorf("forecast: list plans scan: %w", scanErr)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("forecast: list plans rows.Err: %w", err)
	}

	nextCursor := ""
	if len(out) == limit {
		nextCursor = out[len(out)-1].ID.String()
	}
	return out, nextCursor, nil
}

// GetPlanByID — single plan.
func (r *Repository) GetPlanByID(
	ctx context.Context, id uuid.UUID,
) (models.ReplenishmentPlan, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("select_plan_by_id"), id)
	p := models.ReplenishmentPlan{}
	err := row.Scan(
		&p.ID, &p.RunID, &p.SupplierID, &p.LocationID, &p.PlanDate,
		&p.TotalQty, &p.LinesCount, &p.Status, &p.ApprovedAt, &p.ApprovedBy, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ReplenishmentPlan{}, errorspkg.ErrPlanNotFound
		}
		return models.ReplenishmentPlan{}, fmt.Errorf("forecast: get plan: %w", err)
	}
	return p, nil
}

// GetPlanLines — calculation_lines принадлежащие plan'у (по run+supplier+location).
func (r *Repository) GetPlanLines(
	ctx context.Context, runID uuid.UUID, supplierID, locationID string,
) ([]models.CalculationLine, error) {
	rows, err := r.pool.Query(ctx, queries.MustGet("select_plan_lines"),
		runID, supplierID, locationID)
	if err != nil {
		return nil, fmt.Errorf("forecast: get plan lines query: %w", err)
	}
	defer rows.Close()

	out := make([]models.CalculationLine, 0, 64) //nolint:mnd // pre-alloc
	for rows.Next() {
		l := models.CalculationLine{}
		if scanErr := rows.Scan(
			&l.ID, &l.ProductID, &l.LocationID, &l.SupplierID,
			&l.CurrentStock, &l.InTransit, &l.DailyDemand, &l.LeadTimeDays,
			&l.SafetyStock, &l.ReorderPoint, &l.TargetStock, &l.ReorderQty,
		); scanErr != nil {
			return nil, fmt.Errorf("forecast: get plan lines scan: %w", scanErr)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forecast: get plan lines rows.Err: %w", err)
	}
	return out, nil
}

// ApprovePlan — draft → approved. Возвращает ErrPlanNotFound если plan нет
// и ErrPlanNotDraft если статус не draft.
//
// Различие достигается двумя запросами: сначала пробуем UPDATE с фильтром
// status='draft'; если 0 rows — проверяем существует ли plan.
func (r *Repository) ApprovePlan(
	ctx context.Context, id uuid.UUID, approvedBy string,
) (models.ReplenishmentPlan, error) {
	row := r.pool.QueryRow(ctx, queries.MustGet("update_plan_approve"), id, approvedBy)
	p := models.ReplenishmentPlan{}
	err := row.Scan(
		&p.ID, &p.RunID, &p.SupplierID, &p.LocationID, &p.PlanDate,
		&p.TotalQty, &p.LinesCount, &p.Status, &p.ApprovedAt, &p.ApprovedBy, &p.CreatedAt,
	)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return models.ReplenishmentPlan{}, fmt.Errorf("forecast: approve plan: %w", err)
	}
	// 0 rows updated → либо plan нет, либо status не draft.
	existing, getErr := r.GetPlanByID(ctx, id)
	if getErr != nil {
		return models.ReplenishmentPlan{}, getErr
	}
	if existing.Status != "draft" {
		return models.ReplenishmentPlan{}, errorspkg.ErrPlanNotDraft
	}
	// Race: plan был draft, но кто-то другой обновил между UPDATE и SELECT.
	// Возвращаем последнее видимое состояние (idempotent retry успокоит клиента).
	return existing, nil
}
