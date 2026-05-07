package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/engine"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// fakeRepo — in-memory mock для engine tests.
type fakeRepo struct {
	calcRows  []models.CalcInputRow
	demand    []models.DemandPoint
	scores    []models.SupplierScore
	etlRunID  *uuid.UUID
	insertErr error

	forecasts []models.Forecast
	lines     []models.CalculationLine
	plans     []models.ReplenishmentPlan
	committed bool
	failedMsg string
}

func (f *fakeRepo) GetLatestCommittedEtlRunID(_ context.Context) (*uuid.UUID, error) {
	return f.etlRunID, nil
}
func (f *fakeRepo) InsertRun(_ context.Context, _ engine.InsertRunInput) (models.ForecastRun, error) {
	if f.insertErr != nil {
		return models.ForecastRun{}, f.insertErr
	}
	return models.ForecastRun{ID: uuid.New(), Status: "running"}, nil
}
func (f *fakeRepo) UpdateRunCommitted(_ context.Context, _ uuid.UUID, _, _, _ int) error {
	f.committed = true
	return nil
}
func (f *fakeRepo) UpdateRunFailed(_ context.Context, _ uuid.UUID, msg string) error {
	f.failedMsg = msg
	return nil
}
func (f *fakeRepo) ReadCalcInput(_ context.Context, _ *uuid.UUID) ([]models.CalcInputRow, error) {
	return f.calcRows, nil
}
func (f *fakeRepo) ReadDemandWindow(_ context.Context, _, _ time.Time) ([]models.DemandPoint, error) {
	return f.demand, nil
}
func (f *fakeRepo) ReadSupplierScores(_ context.Context, _, _ time.Time) ([]models.SupplierScore, error) {
	return f.scores, nil
}
func (f *fakeRepo) BulkInsertForecasts(
	_ context.Context, _ uuid.UUID, items []models.Forecast, _ string, _ float64,
) error {
	f.forecasts = append(f.forecasts, items...)
	return nil
}
func (f *fakeRepo) BulkInsertCalculationLines(
	_ context.Context, _ uuid.UUID, items []models.CalculationLine,
) error {
	f.lines = append(f.lines, items...)
	return nil
}
func (f *fakeRepo) BulkInsertPlans(
	_ context.Context, _ uuid.UUID, items []models.ReplenishmentPlan,
) error {
	f.plans = append(f.plans, items...)
	return nil
}

func TestEngine_HappyPath(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	supS1 := "s1"
	dd := 5.0
	repo := &fakeRepo{
		calcRows: []models.CalcInputRow{
			{
				ProductID: "p1", LocationID: "l1",
				OnHand: 50, InTransit: 0, DailyDemand: &dd,
				SupplierID: &supS1,
			},
		},
		demand: buildDemand(asOf, 30, 5.0),
	}
	eng := engine.New(repo, nil, nil, nil, nil, nil)

	res, err := eng.Run(context.Background(), engine.RunInput{
		AsOf:        asOf,
		HorizonDays: 14,
	})
	require.NoError(t, err)
	require.True(t, repo.committed)
	require.Greater(t, res.Forecasts, 0)
	require.Greater(t, res.Lines, 0)
	require.Greater(t, res.Plans, 0,
		"existing supplier with positive reorder_qty → plan exists")
}

func TestEngine_EmptyMarts_StillCommits(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		calcRows: nil,
	}
	eng := engine.New(repo, nil, nil, nil, nil, nil)
	res, err := eng.Run(context.Background(), engine.RunInput{AsOf: asOf, HorizonDays: 14})
	require.NoError(t, err)
	require.True(t, repo.committed,
		"empty marts → committed with 0 counters")
	require.Equal(t, 0, res.Forecasts)
	require.Equal(t, 0, res.Lines)
	require.Equal(t, 0, res.Plans)
}

func TestEngine_DefaultHorizon(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		calcRows: []models.CalcInputRow{},
	}
	eng := engine.New(repo, nil, nil, nil, nil, nil)
	_, err := eng.Run(context.Background(), engine.RunInput{
		AsOf:        asOf,
		HorizonDays: 0, // → fallback 14
	})
	require.NoError(t, err)
}

// --- helpers ---

func buildDemand(asOf time.Time, days int, qty float64) []models.DemandPoint {
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
