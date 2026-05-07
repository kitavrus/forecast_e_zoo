package calculators

import (
	"encoding/json"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

// OSAParams — JSON params калибровки OSA.
type OSAParams struct {
	LookbackDays    int `json:"lookback_days"`
	MinObservations int `json:"min_observations"`
}

const (
	osaDefaultLookback        = 30
	osaDefaultMinObservations = 7
)

// ParseOSAParams — JSON → OSAParams; missing/empty fields → defaults.
func ParseOSAParams(raw json.RawMessage) OSAParams {
	p := OSAParams{LookbackDays: osaDefaultLookback, MinObservations: osaDefaultMinObservations}
	if len(raw) == 0 {
		return p
	}
	_ = json.Unmarshal(raw, &p)
	if p.LookbackDays <= 0 {
		p.LookbackDays = osaDefaultLookback
	}
	if p.MinObservations <= 0 {
		p.MinObservations = osaDefaultMinObservations
	}
	return p
}

// ComputeOSA считает per-product_location OSA по агрегациям demand_history.
//
// Формула: OSA = 1 - days_oos / days_observed.
// Если days_observed < min_observations → запись пропускается (data_gap).
//
// Возвращает (snapshots, errorsCount). errorsCount всегда 0 в текущей реализации
// — формула чистая, без побочных эффектов; field оставлен для будущих gates.
func ComputeOSA(rows []models.DemandHistoryAgg, params OSAParams) ([]ComputedSnapshot, int) {
	out := make([]ComputedSnapshot, 0, len(rows))
	for _, r := range rows {
		if r.DaysObserved < params.MinObservations {
			continue
		}
		// нужно избежать деления на 0 — здесь days_observed >= min_observations >= 1.
		osa := 1.0 - (float64(r.DaysOOS) / float64(r.DaysObserved))
		if osa < 0 {
			osa = 0
		}
		key := r.ProductID + "|" + r.LocationID
		k := key
		out = append(out, ComputedSnapshot{
			KpiName:   constants.KpiOSA,
			ScopeType: constants.ScopeTypeProductLocation,
			ScopeID:   &k,
			Value:     osa,
		})
	}
	return out, 0
}
