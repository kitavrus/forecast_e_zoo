package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MartDemandHistory — строка marts.mart_demand_history.
type MartDemandHistory struct {
	AsOfDate              time.Time `db:"as_of_date"`
	LocationID            string    `db:"location_id"`
	ProductID             string    `db:"product_id"`
	QtySold               float64   `db:"qty_sold"`
	QtyReturned           float64   `db:"qty_returned"`
	QtyPromoBonus         float64   `db:"qty_promo_bonus"`
	QtyGift               float64   `db:"qty_gift"`
	RevenuePaid           float64   `db:"revenue_paid"`
	DiscountTotal         float64   `db:"discount_total"`
	TransactionsCount     int       `db:"transactions_count"`
	HadPromo              bool      `db:"had_promo"`
	PromoType             *string   `db:"promo_type"`
	WasInAssortment       bool      `db:"was_in_assortment"`
	LifecycleStateAtDate  *string   `db:"lifecycle_state_at_date"`
	WasOOS                bool      `db:"was_oos"`
	EtlRunID              uuid.UUID `db:"etl_run_id"`
	SourceLoadID          uuid.UUID `db:"source_load_id"`
	CreatedAt             time.Time `db:"created_at"`
}

// MartCalculationInput — строка marts.mart_calculation_input.
type MartCalculationInput struct {
	ProductID            string    `db:"product_id"`
	LocationID           string    `db:"location_id"`
	OnHand               float64   `db:"on_hand"`
	InTransit            float64   `db:"in_transit"`
	SafetyStock          *float64  `db:"safety_stock"`
	ForecastHorizonDays  *int      `db:"forecast_horizon_days"`
	DailyDemand          *float64  `db:"daily_demand"`
	MinQty               *float64  `db:"min_qty"`
	MaxQty               *float64  `db:"max_qty"`
	ApplicableRuleID     *string   `db:"applicable_rule_id"`
	ApplicableRuleKind   string    `db:"applicable_rule_kind"` // order_rule|supply_spec|none
	Formula              *string   `db:"formula"`
	SupplierID           *string   `db:"supplier_id"`
	LeadTimeDays         *int      `db:"lead_time_days"`
	EtlRunID             uuid.UUID `db:"etl_run_id"`
	SourceLoadID         uuid.UUID `db:"source_load_id"`
	CreatedAt            time.Time `db:"created_at"`
}

// MartKpiDaily — строка marts.mart_kpi_daily.
type MartKpiDaily struct {
	AsOfDate     time.Time `db:"as_of_date"`
	LocationID   string    `db:"location_id"`
	KpiName      string    `db:"kpi_name"`
	KpiValue     float64   `db:"kpi_value"`
	KpiUnit      *string   `db:"kpi_unit"`
	EtlRunID     uuid.UUID `db:"etl_run_id"`
	SourceLoadID uuid.UUID `db:"source_load_id"`
	CreatedAt    time.Time `db:"created_at"`
}

// MartMasterCurrent — строка marts.mart_master_current.
type MartMasterCurrent struct {
	EntityType   string          `db:"entity_type"` // product|location|supplier
	EntityID     string          `db:"entity_id"`
	Payload      json.RawMessage `db:"payload"`
	EtlRunID     uuid.UUID       `db:"etl_run_id"`
	SourceLoadID uuid.UUID       `db:"source_load_id"`
	CreatedAt    time.Time       `db:"created_at"`
}

// MartSupplierScorecard — строка marts.mart_supplier_scorecard.
type MartSupplierScorecard struct {
	SupplierID         string    `db:"supplier_id"`
	WeekStart          time.Time `db:"week_start"`
	FillRateAvg        *float64  `db:"fill_rate_avg"`
	OtifPct            *float64  `db:"otif_pct"`
	LeadTimeActualAvg  *float64  `db:"lead_time_actual_avg"`
	QtyShortTotal      float64   `db:"qty_short_total"`
	QtyDamagedTotal    float64   `db:"qty_damaged_total"`
	QtyReturnedTotal   float64   `db:"qty_returned_total"`
	LinesDelivered     int       `db:"lines_delivered"`
	LinesLate          int       `db:"lines_late"`
	EtlRunID           uuid.UUID `db:"etl_run_id"`
	SourceLoadID       uuid.UUID `db:"source_load_id"`
	CreatedAt          time.Time `db:"created_at"`
}
