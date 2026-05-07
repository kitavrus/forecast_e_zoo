// Per-mart select-методы. Каждый возвращает []models.MartRow + nextPK.
package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/sqls/queries"
)

// parsePK3 — парсит cursor.LastPK как "f1|f2|f3" (последний — date).
// Возвращает 3 значения; если меньше — заполняет пустыми/zero-time.
func parsePK3(s string) (p1, p2 string, p3 time.Time) {
	if s == "" {
		return "", "", time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	parts := strings.SplitN(s, "|", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	p1, p2 = parts[0], parts[1]
	if t, err := time.Parse("2006-01-02", parts[2]); err == nil {
		p3 = t
	} else {
		p3 = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return p1, p2, p3
}

// parsePK2Text — парсит "f1|f2" (оба text).
func parsePK2Text(s string) (p1, p2 string) {
	if s == "" {
		return "", ""
	}
	parts := strings.SplitN(s, "|", 2)
	if len(parts) < 2 {
		parts = append(parts, "")
	}
	return parts[0], parts[1]
}

// parsePK2TextDate — парсит "f1|YYYY-MM-DD".
func parsePK2TextDate(s string) (p1 string, p2 time.Time) {
	if s == "" {
		return "", time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	parts := strings.SplitN(s, "|", 2)
	p1 = parts[0]
	if len(parts) >= 2 {
		if t, err := time.Parse("2006-01-02", parts[1]); err == nil {
			return p1, t
		}
	}
	return p1, time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
}

// formatDate — YYYY-MM-DD.
func formatDate(t time.Time) string { return t.Format("2006-01-02") }

// --- mart_demand_history ---

func (r *Repository) selectDemandHistory(
	ctx context.Context, etlRunID uuid.UUID, lastPK string, limit int,
) ([]models.MartRow, string, error) {
	p1, p2, p3 := parsePK3(lastPK)
	rows, err := r.pool.Query(ctx, queries.MustGet("select_mart_demand_history"),
		etlRunID, p1, p2, p3, limit)
	if err != nil {
		return nil, "", fmt.Errorf("data_marts: select demand_history: %w", err)
	}
	defer rows.Close()

	out := make([]models.MartRow, 0, limit)
	var lastProductID, lastLocationID string
	var lastAsOf time.Time
	for rows.Next() {
		var (
			productID, locationID                                    string
			asOfDate                                                 time.Time
			qtySold, qtyReturned, qtyPromoBonus, qtyGift             float64
			revenuePaid, discountTotal                               float64
			transactionsCount                                        int
			hadPromo, wasInAssortment, wasOOS                        bool
			promoType, lifecycleStateAtDate                          *string
			etlRunID, sourceLoadID                                   uuid.UUID
			createdAt                                                time.Time
		)
		if err := rows.Scan(
			&productID, &locationID, &asOfDate,
			&qtySold, &qtyReturned, &qtyPromoBonus, &qtyGift,
			&revenuePaid, &discountTotal, &transactionsCount,
			&hadPromo, &promoType, &wasInAssortment,
			&lifecycleStateAtDate, &wasOOS,
			&etlRunID, &sourceLoadID, &createdAt,
		); err != nil {
			return nil, "", fmt.Errorf("data_marts: scan demand_history: %w", err)
		}
		out = append(out, models.MartRow{
			"product_id": productID, "location_id": locationID, "as_of_date": asOfDate,
			"qty_sold": qtySold, "qty_returned": qtyReturned,
			"qty_promo_bonus": qtyPromoBonus, "qty_gift": qtyGift,
			"revenue_paid": revenuePaid, "discount_total": discountTotal,
			"transactions_count": transactionsCount,
			"had_promo":          hadPromo, "promo_type": promoType,
			"was_in_assortment":       wasInAssortment,
			"lifecycle_state_at_date": lifecycleStateAtDate,
			"was_oos":                 wasOOS,
			"etl_run_id":              etlRunID, "source_load_id": sourceLoadID,
			"created_at": createdAt,
		})
		lastProductID, lastLocationID, lastAsOf = productID, locationID, asOfDate
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("data_marts: rows.Err demand_history: %w", err)
	}
	if len(out) < limit {
		return out, "", nil // последняя страница
	}
	return out, fmt.Sprintf("%s|%s|%s", lastProductID, lastLocationID, formatDate(lastAsOf)), nil
}

// --- mart_calculation_input ---

func (r *Repository) selectCalculationInput(
	ctx context.Context, etlRunID uuid.UUID, lastPK string, limit int,
) ([]models.MartRow, string, error) {
	p1, p2 := parsePK2Text(lastPK)
	rows, err := r.pool.Query(ctx, queries.MustGet("select_mart_calculation_input"),
		etlRunID, p1, p2, limit)
	if err != nil {
		return nil, "", fmt.Errorf("data_marts: select calculation_input: %w", err)
	}
	defer rows.Close()

	out := make([]models.MartRow, 0, limit)
	var lastProductID, lastLocationID string
	for rows.Next() {
		var (
			productID, locationID                                                                  string
			onHand, inTransit                                                                       float64
			safetyStock, forecast7d, forecast14d, rop, minQty, maxQty                              *float64
			applicableRuleID, applicableRuleKind, formula, supplierID                              *string
			leadTimeDays                                                                            *int
			etlRunIDOut, sourceLoadID                                                              uuid.UUID
			createdAt                                                                              time.Time
		)
		if err := rows.Scan(
			&productID, &locationID, &onHand, &inTransit,
			&safetyStock, &forecast7d, &forecast14d, &rop,
			&minQty, &maxQty,
			&applicableRuleID, &applicableRuleKind, &formula, &supplierID, &leadTimeDays,
			&etlRunIDOut, &sourceLoadID, &createdAt,
		); err != nil {
			return nil, "", fmt.Errorf("data_marts: scan calculation_input: %w", err)
		}
		out = append(out, models.MartRow{
			"product_id":           productID,
			"location_id":          locationID,
			"on_hand":              onHand,
			"in_transit":           inTransit,
			"safety_stock":         safetyStock,
			"forecast_qty_7d":      forecast7d,
			"forecast_qty_14d":     forecast14d,
			"rop":                  rop,
			"min_qty":              minQty,
			"max_qty":              maxQty,
			"applicable_rule_id":   applicableRuleID,
			"applicable_rule_kind": applicableRuleKind,
			"formula":              formula,
			"supplier_id":          supplierID,
			"lead_time_days":       leadTimeDays,
			"etl_run_id":           etlRunIDOut,
			"source_load_id":       sourceLoadID,
			"created_at":           createdAt,
		})
		lastProductID, lastLocationID = productID, locationID
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("data_marts: rows.Err calculation_input: %w", err)
	}
	if len(out) < limit {
		return out, "", nil
	}
	return out, lastProductID + "|" + lastLocationID, nil
}

// --- mart_kpi_daily ---

func (r *Repository) selectKpiDaily(
	ctx context.Context, etlRunID uuid.UUID, lastPK string, limit int,
) ([]models.MartRow, string, error) {
	p1, p2, p3 := parsePK3(lastPK)
	rows, err := r.pool.Query(ctx, queries.MustGet("select_mart_kpi_daily"),
		etlRunID, p1, p2, p3, limit)
	if err != nil {
		return nil, "", fmt.Errorf("data_marts: select kpi_daily: %w", err)
	}
	defer rows.Close()

	out := make([]models.MartRow, 0, limit)
	var lastLocationID, lastKPIName string
	var lastAsOf time.Time
	for rows.Next() {
		var (
			asOfDate                  time.Time
			locationID, kpiName       string
			kpiValue                  float64
			kpiUnit                   *string
			etlRunIDOut, sourceLoadID uuid.UUID
			createdAt                 time.Time
		)
		if err := rows.Scan(
			&asOfDate, &locationID, &kpiName, &kpiValue, &kpiUnit,
			&etlRunIDOut, &sourceLoadID, &createdAt,
		); err != nil {
			return nil, "", fmt.Errorf("data_marts: scan kpi_daily: %w", err)
		}
		out = append(out, models.MartRow{
			"as_of_date":     asOfDate,
			"location_id":    locationID,
			"kpi_name":       kpiName,
			"kpi_value":      kpiValue,
			"kpi_unit":       kpiUnit,
			"etl_run_id":     etlRunIDOut,
			"source_load_id": sourceLoadID,
			"created_at":     createdAt,
		})
		lastLocationID, lastKPIName, lastAsOf = locationID, kpiName, asOfDate
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("data_marts: rows.Err kpi_daily: %w", err)
	}
	if len(out) < limit {
		return out, "", nil
	}
	return out, fmt.Sprintf("%s|%s|%s", lastLocationID, lastKPIName, formatDate(lastAsOf)), nil
}

// --- mart_master_current ---

func (r *Repository) selectMasterCurrent(
	ctx context.Context, etlRunID uuid.UUID, lastPK string, limit int,
) ([]models.MartRow, string, error) {
	p1, p2 := parsePK2Text(lastPK)
	rows, err := r.pool.Query(ctx, queries.MustGet("select_mart_master_current"),
		etlRunID, p1, p2, limit)
	if err != nil {
		return nil, "", fmt.Errorf("data_marts: select master_current: %w", err)
	}
	defer rows.Close()

	out := make([]models.MartRow, 0, limit)
	var lastEntityType, lastEntityID string
	for rows.Next() {
		var (
			entityType, entityID      string
			payload                   []byte // jsonb
			etlRunIDOut, sourceLoadID uuid.UUID
			createdAt                 time.Time
		)
		if err := rows.Scan(
			&entityType, &entityID, &payload,
			&etlRunIDOut, &sourceLoadID, &createdAt,
		); err != nil {
			return nil, "", fmt.Errorf("data_marts: scan master_current: %w", err)
		}
		out = append(out, models.MartRow{
			"entity_type":    entityType,
			"entity_id":      entityID,
			"payload":        payload, // raw json bytes — handler сериализует как json.RawMessage
			"etl_run_id":     etlRunIDOut,
			"source_load_id": sourceLoadID,
			"created_at":     createdAt,
		})
		lastEntityType, lastEntityID = entityType, entityID
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("data_marts: rows.Err master_current: %w", err)
	}
	if len(out) < limit {
		return out, "", nil
	}
	return out, lastEntityType + "|" + lastEntityID, nil
}

// --- mart_supplier_scorecard ---

func (r *Repository) selectSupplierScorecard(
	ctx context.Context, etlRunID uuid.UUID, lastPK string, limit int,
) ([]models.MartRow, string, error) {
	p1, p2 := parsePK2TextDate(lastPK)
	rows, err := r.pool.Query(ctx, queries.MustGet("select_mart_supplier_scorecard"),
		etlRunID, p1, p2, limit)
	if err != nil {
		return nil, "", fmt.Errorf("data_marts: select supplier_scorecard: %w", err)
	}
	defer rows.Close()

	out := make([]models.MartRow, 0, limit)
	var lastSupplierID string
	var lastWeekStart time.Time
	for rows.Next() {
		var (
			supplierID                                          string
			weekStart                                           time.Time
			fillRateAvg, otifPct, leadTimeActualAvg             *float64
			qtyShortTotal, qtyDamagedTotal, qtyReturnedTotal    float64
			linesDelivered, linesLate                           int
			etlRunIDOut, sourceLoadID                           uuid.UUID
			createdAt                                           time.Time
		)
		if err := rows.Scan(
			&supplierID, &weekStart,
			&fillRateAvg, &otifPct, &leadTimeActualAvg,
			&qtyShortTotal, &qtyDamagedTotal, &qtyReturnedTotal,
			&linesDelivered, &linesLate,
			&etlRunIDOut, &sourceLoadID, &createdAt,
		); err != nil {
			return nil, "", fmt.Errorf("data_marts: scan supplier_scorecard: %w", err)
		}
		out = append(out, models.MartRow{
			"supplier_id":          supplierID,
			"week_start":           weekStart,
			"fill_rate_avg":        fillRateAvg,
			"otif_pct":             otifPct,
			"lead_time_actual_avg": leadTimeActualAvg,
			"qty_short_total":      qtyShortTotal,
			"qty_damaged_total":    qtyDamagedTotal,
			"qty_returned_total":   qtyReturnedTotal,
			"lines_delivered":      linesDelivered,
			"lines_late":           linesLate,
			"etl_run_id":           etlRunIDOut,
			"source_load_id":       sourceLoadID,
			"created_at":           createdAt,
		})
		lastSupplierID, lastWeekStart = supplierID, weekStart
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("data_marts: rows.Err supplier_scorecard: %w", err)
	}
	if len(out) < limit {
		return out, "", nil
	}
	return out, lastSupplierID + "|" + formatDate(lastWeekStart), nil
}
