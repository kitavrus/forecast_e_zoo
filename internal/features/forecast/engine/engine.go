// Package engine — оркестрация одного forecast-run (Module 5).
//
// Engine.Run:
//  1. InsertRun (status=running) + snapshot etl_run_id.
//  2. ReadCalcInput из marts.
//  3. ReadDemandWindow для построения SMA.
//  4. Forecaster.PredictBatch.
//  5. ReadSupplierScores → fallback lead_time.
//  6. Calculator.ComputeBatch.
//  7. Constructor.BuildPlans.
//  8. Bulk-insert (forecasts/lines/plans).
//  9. UpdateRunCommitted.
//
// На любой ошибке после InsertRun → UpdateRunFailed + return.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/calculator"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/constants"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/constructor"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/forecaster"
	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// Repo — узкий интерфейс repository для engine.
type Repo interface {
	GetLatestCommittedEtlRunID(ctx context.Context) (*uuid.UUID, error)
	InsertRun(ctx context.Context, in models.InsertRunInput) (models.ForecastRun, error)
	UpdateRunCommitted(ctx context.Context, runID uuid.UUID, forecasts, lines, plans int) error
	UpdateRunFailed(ctx context.Context, runID uuid.UUID, msg string) error
	ReadCalcInput(ctx context.Context, etlRunID *uuid.UUID) ([]models.CalcInputRow, error)
	ReadDemandWindow(ctx context.Context, from, to time.Time) ([]models.DemandPoint, error)
	ReadSupplierScores(ctx context.Context, from, to time.Time) ([]models.SupplierScore, error)
	BulkInsertForecasts(ctx context.Context, runID uuid.UUID, items []models.Forecast,
		modelName string, confidence float64) error
	BulkInsertCalculationLines(ctx context.Context, runID uuid.UUID,
		items []models.CalculationLine) error
	BulkInsertPlans(ctx context.Context, runID uuid.UUID,
		items []models.ReplenishmentPlan) error
}

// InsertRunInput — alias моделевого типа, для совместимости тестов.
type InsertRunInput = models.InsertRunInput

// Metrics — обёртка над Prometheus.
type Metrics interface {
	IncRun(result string)
	ObserveDuration(seconds float64)
	IncForecasts(count int)
	IncLines(count int)
	IncPlans(count int)
	IncError(reason string)
}

type noopMetrics struct{}

func (noopMetrics) IncRun(string)           {}
func (noopMetrics) ObserveDuration(float64) {}
func (noopMetrics) IncForecasts(int)        {}
func (noopMetrics) IncLines(int)            {}
func (noopMetrics) IncPlans(int)            {}
func (noopMetrics) IncError(string)         {}

// Engine — orchestrator.
type Engine struct {
	repo        Repo
	forecaster  forecaster.Forecaster
	calculator  *calculator.Calculator
	constructor *constructor.Constructor
	logger      *slog.Logger
	metrics     Metrics
}

// New собирает Engine.
func New(
	repo Repo,
	fcr forecaster.Forecaster,
	calc *calculator.Calculator,
	ctor *constructor.Constructor,
	logger *slog.Logger,
	metrics Metrics,
) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	if metrics == nil {
		metrics = noopMetrics{}
	}
	if calc == nil {
		calc = calculator.New()
	}
	if ctor == nil {
		ctor = constructor.New()
	}
	if fcr == nil {
		fcr = forecaster.NewMovingAverageForecaster()
	}
	return &Engine{
		repo:        repo,
		forecaster:  fcr,
		calculator:  calc,
		constructor: ctor,
		logger:      logger,
		metrics:     metrics,
	}
}

// RunInput — параметры запуска.
//
// PresetRunID — если не uuid.Nil, engine не делает InsertRun (caller уже
// вставил forecast_run строку и хочет, чтобы engine использовал тот же ID).
// Без этого scheduler.TryTrigger возвращал клиенту runID, который никогда
// не персистился (engine генерил свой), что ломало poll API после POST.
type RunInput struct {
	AsOf        time.Time
	HorizonDays int
	PresetRunID uuid.UUID
}

// RunResult — итог.
type RunResult struct {
	RunID          uuid.UUID
	Forecasts      int
	Lines          int
	Plans          int
	SnapshotEtlRun *uuid.UUID
}

// ErrNoMartsData — marts пусты, run committed без данных (не ошибка).
var ErrNoMartsData = errors.New("forecast: no calc input rows")

// Run — основная точка.
//
//nolint:funlen,cyclop // orchestration по природе длинная
func (e *Engine) Run(ctx context.Context, in RunInput) (RunResult, error) {
	start := time.Now()
	defer func() { e.metrics.ObserveDuration(time.Since(start).Seconds()) }()

	horizon := in.HorizonDays
	if horizon <= 0 {
		horizon = constants.HorizonDefault
	}
	asOf := in.AsOf.UTC().Truncate(24 * time.Hour) //nolint:mnd

	snapshotID, err := e.repo.GetLatestCommittedEtlRunID(ctx)
	if err != nil {
		e.metrics.IncRun("error")
		return RunResult{}, fmt.Errorf("forecast engine: get etl_run: %w", err)
	}

	var run models.ForecastRun
	if in.PresetRunID != uuid.Nil {
		// Caller (scheduler.TryTrigger) уже вставил forecast_run row
		// синхронно — используем готовый ID.
		run = models.ForecastRun{ID: in.PresetRunID}
	} else {
		run, err = e.repo.InsertRun(ctx, models.InsertRunInput{
			HorizonDays:      horizon,
			SnapshotEtlRunID: snapshotID,
		})
		if err != nil {
			e.metrics.IncRun("error")
			return RunResult{}, fmt.Errorf("forecast engine: insert run: %w", err)
		}
	}

	res, runErr := e.runInner(ctx, run.ID, snapshotID, asOf, horizon)
	if runErr != nil {
		_ = e.repo.UpdateRunFailed(ctx, run.ID, runErr.Error())
		e.metrics.IncRun("error")
		e.metrics.IncError("run_failed")
		e.logger.ErrorContext(ctx, "forecast engine: run failed",
			slog.String("run_id", run.ID.String()),
			slog.Any("error", runErr),
		)
		return RunResult{RunID: run.ID, SnapshotEtlRun: snapshotID}, runErr
	}

	if err := e.repo.UpdateRunCommitted(ctx, run.ID, res.Forecasts, res.Lines, res.Plans); err != nil {
		e.metrics.IncRun("error")
		return res, fmt.Errorf("forecast engine: update committed: %w", err)
	}
	res.RunID = run.ID
	res.SnapshotEtlRun = snapshotID
	e.metrics.IncRun("ok")
	e.metrics.IncForecasts(res.Forecasts)
	e.metrics.IncLines(res.Lines)
	e.metrics.IncPlans(res.Plans)
	e.logger.InfoContext(ctx, "forecast engine: run committed",
		slog.String("run_id", run.ID.String()),
		slog.Int("forecasts", res.Forecasts),
		slog.Int("lines", res.Lines),
		slog.Int("plans", res.Plans),
	)
	return res, nil
}

// runInner — фактическая работа после insert run.
//
//nolint:funlen,cyclop // pipeline orchestration
func (e *Engine) runInner(
	ctx context.Context, runID uuid.UUID, _ *uuid.UUID, asOf time.Time, horizon int,
) (RunResult, error) {
	calcRows, err := e.repo.ReadCalcInput(ctx, nil)
	if err != nil {
		return RunResult{}, fmt.Errorf("read calc input: %w", err)
	}
	if len(calcRows) == 0 {
		return RunResult{}, nil
	}

	from := asOf.AddDate(0, 0, -constants.LookbackDays)
	to := asOf.AddDate(0, 0, -1)
	demand, err := e.repo.ReadDemandWindow(ctx, from, to)
	if err != nil {
		return RunResult{}, fmt.Errorf("read demand window: %w", err)
	}
	forecasts, err := e.forecaster.PredictBatch(ctx, demand, asOf, horizon)
	if err != nil {
		return RunResult{}, fmt.Errorf("forecaster predict: %w", err)
	}

	scoresFrom := asOf.AddDate(0, 0, -28) //nolint:mnd // 4 weeks
	scores, err := e.repo.ReadSupplierScores(ctx, scoresFrom, asOf)
	if err != nil {
		return RunResult{}, fmt.Errorf("read scorecard: %w", err)
	}
	leadTimeMap := buildLeadTimeMap(scores)

	dailyDemandMap := buildDailyDemandMap(forecasts, horizon)

	inputs := buildCalcInputs(calcRows, dailyDemandMap, leadTimeMap)
	lines := e.calculator.ComputeBatch(inputs)

	plans := e.constructor.BuildPlans(lines, nil, asOf)

	if len(forecasts) > 0 {
		if err := e.repo.BulkInsertForecasts(ctx, runID, forecasts,
			e.forecasterModelName(), constants.ConfidenceMVP); err != nil {
			return RunResult{}, fmt.Errorf("bulk insert forecasts: %w", err)
		}
	}
	if len(lines) > 0 {
		if err := e.repo.BulkInsertCalculationLines(ctx, runID, lines); err != nil {
			return RunResult{}, fmt.Errorf("bulk insert lines: %w", err)
		}
	}
	if len(plans) > 0 {
		if err := e.repo.BulkInsertPlans(ctx, runID, plans); err != nil {
			return RunResult{}, fmt.Errorf("bulk insert plans: %w", err)
		}
	}

	return RunResult{
		Forecasts: len(forecasts),
		Lines:     len(lines),
		Plans:     len(plans),
	}, nil
}

func (e *Engine) forecasterModelName() string {
	if mn, ok := e.forecaster.(forecaster.ModelName); ok {
		return mn.ModelName()
	}
	return constants.ModelSMASeasonal
}

func buildLeadTimeMap(scores []models.SupplierScore) map[string]int {
	out := make(map[string]int, len(scores))
	for _, s := range scores {
		if s.LeadTimeActualAvg != nil && *s.LeadTimeActualAvg > 0 {
			out[s.SupplierID] = int(*s.LeadTimeActualAvg + 0.5)
		}
	}
	return out
}

// buildDailyDemandMap — усредняем прогноз на горизонте → daily_demand для калькулятора.
func buildDailyDemandMap(forecasts []models.Forecast, horizon int) map[string]float64 {
	type key struct{ p, l string }
	sums := make(map[key]float64, 256) //nolint:mnd
	counts := make(map[key]int, 256)   //nolint:mnd
	for _, f := range forecasts {
		k := key{f.ProductID, f.LocationID}
		sums[k] += f.ForecastQty
		counts[k]++
	}
	out := make(map[string]float64, len(sums))
	for k, sum := range sums {
		cnt := counts[k]
		if cnt == 0 {
			cnt = horizon
		}
		out[k.p+"|"+k.l] = sum / float64(cnt)
	}
	return out
}

func buildCalcInputs(
	rows []models.CalcInputRow,
	dailyDemand map[string]float64,
	leadTimeMap map[string]int,
) []calculator.Input {
	out := make([]calculator.Input, 0, len(rows))
	for _, r := range rows {
		key := r.ProductID + "|" + r.LocationID
		demand, ok := dailyDemand[key]
		if !ok {
			if r.DailyDemand != nil {
				demand = *r.DailyDemand
			}
		}
		leadTime := constants.LeadTimeDefault
		if r.LeadTimeDays != nil && *r.LeadTimeDays > 0 {
			leadTime = *r.LeadTimeDays
		} else if r.SupplierID != nil {
			if v, ok := leadTimeMap[*r.SupplierID]; ok && v > 0 {
				leadTime = v
			}
		}
		out = append(out, calculator.Input{
			ProductID:    r.ProductID,
			LocationID:   r.LocationID,
			SupplierID:   r.SupplierID,
			CurrentStock: r.OnHand,
			InTransit:    r.InTransit,
			DailyDemand:  demand,
			LeadTimeDays: leadTime,
		})
	}
	return out
}
