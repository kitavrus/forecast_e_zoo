package service

import (
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
)

// schemas — hardcoded mart schemas (ADR-002).
// Изменение требует PR + code review.
//
// Соответствует миграции 1001_marts_schema.up.sql Модуля 2.
var schemas = map[string][]models.MartField{
	constants.MartDemandHistory: {
		{Name: "as_of_date", Type: "date"},
		{Name: "location_id", Type: "text"},
		{Name: "product_id", Type: "text"},
		{Name: "qty_sold", Type: "numeric"},
		{Name: "qty_returned", Type: "numeric"},
		{Name: "qty_promo_bonus", Type: "numeric"},
		{Name: "qty_gift", Type: "numeric"},
		{Name: "revenue_paid", Type: "numeric"},
		{Name: "discount_total", Type: "numeric"},
		{Name: "transactions_count", Type: "integer"},
		{Name: "had_promo", Type: "boolean"},
		{Name: "promo_type", Type: "text"},
		{Name: "was_in_assortment", Type: "boolean"},
		{Name: "lifecycle_state_at_date", Type: "text"},
		{Name: "was_oos", Type: "boolean"},
		{Name: "etl_run_id", Type: "uuid"},
		{Name: "source_load_id", Type: "uuid"},
		{Name: "created_at", Type: "timestamptz"},
	},
	constants.MartCalculationInput: {
		{Name: "product_id", Type: "text"},
		{Name: "location_id", Type: "text"},
		{Name: "on_hand", Type: "numeric"},
		{Name: "in_transit", Type: "numeric"},
		{Name: "safety_stock", Type: "numeric"},
		{Name: "forecast_qty_7d", Type: "numeric"},
		{Name: "forecast_qty_14d", Type: "numeric"},
		{Name: "rop", Type: "numeric"},
		{Name: "min_qty", Type: "numeric"},
		{Name: "max_qty", Type: "numeric"},
		{Name: "applicable_rule_id", Type: "text"},
		{Name: "applicable_rule_kind", Type: "text"},
		{Name: "formula", Type: "text"},
		{Name: "supplier_id", Type: "text"},
		{Name: "lead_time_days", Type: "integer"},
		{Name: "etl_run_id", Type: "uuid"},
		{Name: "source_load_id", Type: "uuid"},
		{Name: "created_at", Type: "timestamptz"},
	},
	constants.MartKpiDaily: {
		{Name: "as_of_date", Type: "date"},
		{Name: "location_id", Type: "text"},
		{Name: "kpi_name", Type: "text"},
		{Name: "kpi_value", Type: "numeric"},
		{Name: "kpi_unit", Type: "text"},
		{Name: "etl_run_id", Type: "uuid"},
		{Name: "source_load_id", Type: "uuid"},
		{Name: "created_at", Type: "timestamptz"},
	},
	constants.MartMasterCurrent: {
		{Name: "entity_type", Type: "text"},
		{Name: "entity_id", Type: "text"},
		{Name: "payload", Type: "jsonb"},
		{Name: "etl_run_id", Type: "uuid"},
		{Name: "source_load_id", Type: "uuid"},
		{Name: "created_at", Type: "timestamptz"},
	},
	constants.MartSupplierScorecard: {
		{Name: "supplier_id", Type: "text"},
		{Name: "week_start", Type: "date"},
		{Name: "fill_rate_avg", Type: "numeric"},
		{Name: "otif_pct", Type: "numeric"},
		{Name: "lead_time_actual_avg", Type: "numeric"},
		{Name: "qty_short_total", Type: "numeric"},
		{Name: "qty_damaged_total", Type: "numeric"},
		{Name: "qty_returned_total", Type: "numeric"},
		{Name: "lines_delivered", Type: "integer"},
		{Name: "lines_late", Type: "integer"},
		{Name: "etl_run_id", Type: "uuid"},
		{Name: "source_load_id", Type: "uuid"},
		{Name: "created_at", Type: "timestamptz"},
	},
}
