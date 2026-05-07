package calculators

import (
	"encoding/json"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

// StockDaysParams — JSON params калибровки stock_days.
type StockDaysParams struct {
	IncludeInTransit bool    `json:"include_in_transit"`
	MinDailyDemand   float64 `json:"min_daily_demand"`
	CapDays          float64 `json:"cap_days"`
}

const (
	stockDaysDefaultMinDailyDemand = 0.001
	stockDaysDefaultCapDays        = 365.0
)

// ParseStockDaysParams — JSON → StockDaysParams с дефолтами.
func ParseStockDaysParams(raw json.RawMessage) StockDaysParams {
	p := StockDaysParams{
		IncludeInTransit: true,
		MinDailyDemand:   stockDaysDefaultMinDailyDemand,
		CapDays:          stockDaysDefaultCapDays,
	}
	if len(raw) == 0 {
		return p
	}
	_ = json.Unmarshal(raw, &p)
	if p.MinDailyDemand <= 0 {
		p.MinDailyDemand = stockDaysDefaultMinDailyDemand
	}
	if p.CapDays <= 0 {
		p.CapDays = stockDaysDefaultCapDays
	}
	return p
}

// ComputeStockDays считает per-(product, location) stock_days.
//
// Формула:
//
//	stock = on_hand (+ in_transit, если IncludeInTransit)
//	demand = max(daily_demand, min_daily_demand)   // защита от деления на 0
//	stock_days = min(stock / demand, cap_days)
//
// Если daily_demand IS NULL — берём min_daily_demand (cap → cap_days).
func ComputeStockDays(rows []models.CalcInputRow, params StockDaysParams) ([]ComputedSnapshot, int) {
	out := make([]ComputedSnapshot, 0, len(rows))
	for _, r := range rows {
		stock := r.OnHand
		if params.IncludeInTransit {
			stock += r.InTransit
		}
		demand := params.MinDailyDemand
		if r.DailyDemand != nil && *r.DailyDemand > demand {
			demand = *r.DailyDemand
		}
		days := stock / demand
		if days > params.CapDays {
			days = params.CapDays
		}
		if days < 0 {
			days = 0
		}
		key := r.ProductID + "|" + r.LocationID
		k := key
		out = append(out, ComputedSnapshot{
			KpiName:   constants.KpiStockDays,
			ScopeType: constants.ScopeTypeProductLocation,
			ScopeID:   &k,
			Value:     days,
		})
	}
	return out, 0
}
