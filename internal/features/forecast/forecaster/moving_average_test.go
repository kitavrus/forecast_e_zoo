package forecaster_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/constants"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/forecaster"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

func TestMovingAverage_HappyPath_FlatDemand(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	history := buildFlatHistory(asOf, 30, 10.0)

	f := forecaster.NewMovingAverageForecaster()
	out, err := f.PredictBatch(context.Background(), history, asOf, 14)
	require.NoError(t, err)
	require.Len(t, out, 14, "14 days × 1 group")

	for _, fc := range out {
		require.InDelta(t, 10.0, fc.ForecastQty, 0.001,
			"flat demand should yield avg = 10")
		require.Equal(t, constants.ModelSMASeasonal, fc.ModelName)
	}
}

func TestMovingAverage_DOWMultiplier(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC) // Thursday
	// Истоки: пн-чт по 5 шт, пт-вс по 20 шт.
	history := buildWeekendBoostedHistory(asOf, 28)

	f := forecaster.NewMovingAverageForecaster()
	out, err := f.PredictBatch(context.Background(), history, asOf, 7)
	require.NoError(t, err)
	require.Len(t, out, 7)

	// Проверяем: сб/вс прогноз > среднее, пн прогноз < среднее.
	var weekendQty, weekdayQty float64
	for _, fc := range out {
		dow := fc.ForecastDate.Weekday()
		if dow == time.Saturday || dow == time.Sunday {
			weekendQty = fc.ForecastQty
		}
		if dow == time.Monday {
			weekdayQty = fc.ForecastQty
		}
	}
	require.Greater(t, weekendQty, weekdayQty,
		"DOW multiplier should boost weekend forecasts")
}

func TestMovingAverage_EmptyHistory(t *testing.T) {
	t.Parallel()
	f := forecaster.NewMovingAverageForecaster()
	out, err := f.PredictBatch(context.Background(), nil,
		time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC), 14)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestMovingAverage_ZeroHorizon(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	hist := buildFlatHistory(asOf, 7, 5.0)
	f := forecaster.NewMovingAverageForecaster()
	out, err := f.PredictBatch(context.Background(), hist, asOf, 0)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestMovingAverage_BoundsAndConfidence(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	hist := buildFlatHistory(asOf, 14, 100.0)
	f := forecaster.NewMovingAverageForecaster()
	out, err := f.PredictBatch(context.Background(), hist, asOf, 1)
	require.NoError(t, err)
	require.Len(t, out, 1)
	fc := out[0]
	require.NotNil(t, fc.LowerBound)
	require.NotNil(t, fc.UpperBound)
	require.NotNil(t, fc.Confidence)
	require.Less(t, *fc.LowerBound, fc.ForecastQty)
	require.Greater(t, *fc.UpperBound, fc.ForecastQty)
	require.InDelta(t, constants.ConfidenceMVP, *fc.Confidence, 0.001)
}

// --- helpers ---

func buildFlatHistory(asOf time.Time, days int, qty float64) []models.DemandPoint {
	out := make([]models.DemandPoint, 0, days)
	for i := 1; i <= days; i++ {
		out = append(out, models.DemandPoint{
			ProductID:  "p1",
			LocationID: "l1",
			AsOfDate:   asOf.AddDate(0, 0, -i),
			QtySold:    qty,
		})
	}
	return out
}

func buildWeekendBoostedHistory(asOf time.Time, days int) []models.DemandPoint {
	out := make([]models.DemandPoint, 0, days)
	for i := 1; i <= days; i++ {
		d := asOf.AddDate(0, 0, -i)
		qty := 5.0
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || d.Weekday() == time.Friday {
			qty = 20.0
		}
		out = append(out, models.DemandPoint{
			ProductID:  "p1",
			LocationID: "l1",
			AsOfDate:   d,
			QtySold:    qty,
		})
	}
	return out
}
