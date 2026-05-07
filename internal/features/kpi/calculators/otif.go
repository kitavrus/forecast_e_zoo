package calculators

import (
	"encoding/json"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

// OTIFParams — JSON params калибровки OTIF.
type OTIFParams struct {
	LateGraceHours    int     `json:"late_grace_hours"`
	FillRateThreshold float64 `json:"fill_rate_threshold"`
}

const (
	otifDefaultLateGrace      = 0
	otifDefaultFillThreshold  = 0.95
)

// ParseOTIFParams — JSON → OTIFParams с дефолтами.
func ParseOTIFParams(raw json.RawMessage) OTIFParams {
	p := OTIFParams{LateGraceHours: otifDefaultLateGrace, FillRateThreshold: otifDefaultFillThreshold}
	if len(raw) == 0 {
		return p
	}
	_ = json.Unmarshal(raw, &p)
	if p.FillRateThreshold <= 0 || p.FillRateThreshold > 1 {
		p.FillRateThreshold = otifDefaultFillThreshold
	}
	if p.LateGraceHours < 0 {
		p.LateGraceHours = 0
	}
	return p
}

// ComputeOTIF считает per-supplier OTIF по строкам supplier_scorecard.
//
// Формула:
//
//	otif = 1 - (lines_late + lines_short) / lines_delivered
//
// где lines_short = lines_delivered - on_time_full_lines, оценивается через
// fill_rate_avg < fill_rate_threshold.
//
// Если lines_delivered=0 → запись пропускается (no data).
func ComputeOTIF(rows []models.SupplierScorecardRow, params OTIFParams) ([]ComputedSnapshot, int) {
	out := make([]ComputedSnapshot, 0, len(rows))
	errs := 0
	for _, r := range rows {
		if r.LinesDelivered <= 0 {
			continue
		}
		// short approximation: если fill_rate_avg ниже threshold,
		// весь scorecard week считается short.
		linesShort := 0
		if r.FillRateAvg != nil && *r.FillRateAvg < params.FillRateThreshold {
			linesShort = r.LinesDelivered
		}
		bad := r.LinesLate + linesShort
		if bad > r.LinesDelivered {
			bad = r.LinesDelivered
		}
		otif := 1.0 - (float64(bad) / float64(r.LinesDelivered))
		if otif < 0 {
			errs++
			continue
		}
		sid := r.SupplierID
		out = append(out, ComputedSnapshot{
			KpiName:   constants.KpiOTIF,
			ScopeType: constants.ScopeTypeSupplier,
			ScopeID:   &sid,
			Value:     otif,
		})
	}
	return out, errs
}
