package forecaster

import (
	"context"
	"math"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/constants"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// MovingAverageForecaster — SMA(lookback_days) + DOW multiplier + WOY multiplier.
//
// Формула:
//   forecast(d) = avg_last_N_days × dow_multiplier(weekday(d)) × woy_multiplier(week_of_year(d))
// где
//   dow_multiplier = avg_demand_per_weekday / overall_avg
//   woy_multiplier = avg_demand_per_weekofyear / overall_avg (если ≥4 сезона, иначе 1.0)
//
// Если для (product, location) недостаточно данных (lookback_days < 7) —
// прогноз = последний наблюдаемый qty_sold (или 0).
type MovingAverageForecaster struct {
	lookbackDays  int
	woyMinSeasons int
}

// NewMovingAverageForecaster — конструктор с дефолтами.
func NewMovingAverageForecaster() *MovingAverageForecaster {
	return &MovingAverageForecaster{
		lookbackDays:  constants.LookbackDays,
		woyMinSeasons: 4, //nolint:mnd // конкретное число сезонов для WOY
	}
}

// ModelName — имя для записи в forecasts.model_name.
func (m *MovingAverageForecaster) ModelName() string {
	return constants.ModelSMASeasonal
}

// PredictBatch реализует Forecaster.
func (m *MovingAverageForecaster) PredictBatch(
	_ context.Context,
	history []models.DemandPoint,
	asOf time.Time,
	horizonDays int,
) ([]models.Forecast, error) {
	if horizonDays <= 0 || len(history) == 0 {
		return []models.Forecast{}, nil
	}
	groups := groupBy(history)
	out := make([]models.Forecast, 0, len(groups)*horizonDays)
	asOfDate := asOf.UTC().Truncate(24 * time.Hour) //nolint:mnd // 1 day truncation

	for key, points := range groups {
		stats := computeStats(points)
		for i := 0; i < horizonDays; i++ {
			date := asOfDate.AddDate(0, 0, i)
			qty := m.predict(stats, date)
			lower := qty * (1 - constants.ConfidenceBoundDelta)
			upper := qty * (1 + constants.ConfidenceBoundDelta)
			conf := constants.ConfidenceMVP
			out = append(out, models.Forecast{
				ProductID:    key.product,
				LocationID:   key.location,
				ForecastDate: date,
				ForecastQty:  qty,
				LowerBound:   &lower,
				UpperBound:   &upper,
				ModelName:    m.ModelName(),
				Confidence:   &conf,
			})
		}
	}
	return out, nil
}

func (m *MovingAverageForecaster) predict(s seriesStats, date time.Time) float64 {
	if s.observations == 0 || s.average == 0 {
		return 0
	}
	dow := int(date.Weekday())
	dowMul := 1.0
	if cnt, ok := s.dowCount[dow]; ok && cnt > 0 {
		dowMul = (s.dowSum[dow] / float64(cnt)) / s.average
	}
	woyMul := 1.0
	if s.woySeasonsCount >= m.woyMinSeasons {
		_, woy := date.ISOWeek()
		if cnt, ok := s.woyCount[woy]; ok && cnt > 0 {
			woyMul = (s.woySum[woy] / float64(cnt)) / s.average
		}
	}
	q := s.average * dowMul * woyMul
	if q < 0 {
		q = 0
	}
	if math.IsNaN(q) || math.IsInf(q, 0) {
		return 0
	}
	return q
}

type plKey struct {
	product, location string
}

type seriesStats struct {
	observations    int
	average         float64
	dowSum          map[int]float64
	dowCount        map[int]int
	woySum          map[int]float64
	woyCount        map[int]int
	woySeasonsCount int // распределённых по разным WOY (для условия ≥4)
}

func groupBy(history []models.DemandPoint) map[plKey][]models.DemandPoint {
	groups := make(map[plKey][]models.DemandPoint, 256) //nolint:mnd // pre-alloc
	for _, p := range history {
		k := plKey{p.ProductID, p.LocationID}
		groups[k] = append(groups[k], p)
	}
	return groups
}

func computeStats(points []models.DemandPoint) seriesStats {
	s := seriesStats{
		dowSum:   make(map[int]float64, 7),  //nolint:mnd
		dowCount: make(map[int]int, 7),      //nolint:mnd
		woySum:   make(map[int]float64, 53), //nolint:mnd
		woyCount: make(map[int]int, 53),     //nolint:mnd
	}
	if len(points) == 0 {
		return s
	}
	var sum float64
	woySeen := make(map[int]struct{}, 53) //nolint:mnd
	for _, p := range points {
		sum += p.QtySold
		dow := int(p.AsOfDate.Weekday())
		s.dowSum[dow] += p.QtySold
		s.dowCount[dow]++
		_, woy := p.AsOfDate.ISOWeek()
		s.woySum[woy] += p.QtySold
		s.woyCount[woy]++
		woySeen[woy] = struct{}{}
	}
	s.observations = len(points)
	s.average = sum / float64(len(points))
	s.woySeasonsCount = len(woySeen)
	return s
}
