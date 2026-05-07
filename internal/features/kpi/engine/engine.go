// Package engine — оркестрация KPI run.
//
// Engine.Run:
//  1. Грузит все калибровки → Resolver.
//  2. Параллельно (или последовательно — MVP последовательно) для каждого KPI:
//     - читает marts.* (через repo);
//     - расчитывает калькулятором;
//     - проверяет quality threshold;
//     - пишет в kpi_snapshots.
//  3. Возвращает Run summary (per-KPI status).
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/calculators"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/calibration"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/repository"
)

// Repo — узкий интерфейс для тестируемости engine.
type Repo interface {
	ListCalibrations(ctx context.Context, f models.CalibrationFilter) ([]models.KpiCalibration, error)
	ReadDemandHistoryAggregates(ctx context.Context, from, to time.Time) ([]models.DemandHistoryAgg, error)
	ReadCalculationInput(ctx context.Context) ([]models.CalcInputRow, error)
	ReadSupplierScorecard(ctx context.Context, from, to time.Time) ([]models.SupplierScorecardRow, error)
	InsertSnapshot(ctx context.Context, in repository.InsertSnapshotInput) (models.KpiSnapshot, error)
	DeleteSnapshotsForDate(ctx context.Context, date time.Time, kpiNames []string) (int64, error)
}

// Metrics — обёртка над Prometheus, чтобы engine не знал про конкретные счётчики.
type Metrics interface {
	IncRun(result string)
	ObserveDuration(seconds float64)
	IncSnapshot(kpiName string, count int)
	IncError(kpiName, reason string)
}

// noopMetrics — заглушка для тестов.
type noopMetrics struct{}

func (noopMetrics) IncRun(string)               {}
func (noopMetrics) ObserveDuration(float64)     {}
func (noopMetrics) IncSnapshot(string, int)     {}
func (noopMetrics) IncError(string, string)     {}

// Engine оркестрирует KPI расчёты.
type Engine struct {
	repo    Repo
	logger  *slog.Logger
	metrics Metrics
}

// New создаёт Engine.
func New(repo Repo, logger *slog.Logger, metrics Metrics) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	if metrics == nil {
		metrics = noopMetrics{}
	}
	return &Engine{repo: repo, logger: logger, metrics: metrics}
}

// RunInput — параметры одного запуска engine.
type RunInput struct {
	RunID    uuid.UUID
	AsOfDate time.Time
	KpiNames []string // nil = все три KPI
	// EtlRunID — опционально; пишется в snapshots для traceability.
	EtlRunID *uuid.UUID
}

// RunResult — суммарный итог.
type RunResult struct {
	RunID    uuid.UUID
	Snapshot map[string]int // kpi_name → snapshots written
	Errors   map[string]error
}

// ErrQualityThreshold — превышен 5% бюджет ошибок.
var ErrQualityThreshold = errors.New("kpi: quality error budget exceeded")

// Run — основная точка. Возвращает агрегированный результат и nil ошибку,
// если хотя бы один KPI отработал; иначе оборачивает первую критическую ошибку.
func (e *Engine) Run(ctx context.Context, in RunInput) (RunResult, error) {
	start := time.Now()
	defer func() { e.metrics.ObserveDuration(time.Since(start).Seconds()) }()

	kpis := in.KpiNames
	if len(kpis) == 0 {
		kpis = []string{constants.KpiOSA, constants.KpiOTIF, constants.KpiStockDays}
	}

	// Удаляем существующие снапшоты за дату — refresh-семантика.
	if _, err := e.repo.DeleteSnapshotsForDate(ctx, in.AsOfDate, kpis); err != nil {
		e.metrics.IncRun("error")
		return RunResult{}, fmt.Errorf("kpi engine: delete pre-run: %w", err)
	}

	all, err := e.repo.ListCalibrations(ctx, models.CalibrationFilter{})
	if err != nil {
		e.metrics.IncRun("error")
		return RunResult{}, fmt.Errorf("kpi engine: load calibrations: %w", err)
	}
	resolver := calibration.NewResolver(all)

	res := RunResult{
		RunID:    in.RunID,
		Snapshot: make(map[string]int, len(kpis)),
		Errors:   make(map[string]error),
	}

	for _, kpi := range kpis {
		count, err := e.runKpi(ctx, kpi, in, resolver)
		res.Snapshot[kpi] = count
		if err != nil {
			res.Errors[kpi] = err
			e.metrics.IncError(kpi, "compute")
			e.logger.ErrorContext(ctx, "kpi engine: kpi failed",
				slog.String("kpi", kpi),
				slog.Any("error", err),
			)
			continue
		}
		e.metrics.IncSnapshot(kpi, count)
	}

	if len(res.Errors) == len(kpis) {
		e.metrics.IncRun("error")
		return res, fmt.Errorf("kpi engine: all kpis failed")
	}
	if len(res.Errors) > 0 {
		e.metrics.IncRun("partial")
		return res, nil
	}
	e.metrics.IncRun("ok")
	return res, nil
}

// runKpi выполняет один KPI и возвращает (snapshots_written, err).
func (e *Engine) runKpi(
	ctx context.Context, kpi string, in RunInput, resolver *calibration.Resolver,
) (int, error) {
	snaps, errsCount, totalCandidates, err := e.computeKpi(ctx, kpi, in, resolver)
	if err != nil {
		return 0, err
	}
	if totalCandidates > 0 && float64(errsCount)/float64(totalCandidates) > constants.QualityErrorBudget {
		return 0, fmt.Errorf("%w: kpi=%s errors=%d total=%d",
			ErrQualityThreshold, kpi, errsCount, totalCandidates)
	}
	written := 0
	for _, s := range snaps {
		calibID := matchedCalibrationID(resolver, s)
		var calibPtr *uuid.UUID
		if calibID != uuid.Nil {
			c := calibID
			calibPtr = &c
		}
		if _, err := e.repo.InsertSnapshot(ctx, repository.InsertSnapshotInput{
			AsOfDate:      in.AsOfDate,
			KpiName:       s.KpiName,
			ScopeType:     s.ScopeType,
			ScopeID:       s.ScopeID,
			Value:         s.Value,
			CalibrationID: calibPtr,
			EtlRunID:      in.EtlRunID,
		}); err != nil {
			return written, fmt.Errorf("kpi engine: insert snapshot %s: %w", kpi, err)
		}
		written++
	}
	return written, nil
}

// computeKpi выбирает калькулятор и возвращает (snapshots, errs, total candidates, err).
//
//nolint:cyclop // switch по 3 фиксированным KPI
func (e *Engine) computeKpi(
	ctx context.Context, kpi string, in RunInput, resolver *calibration.Resolver,
) ([]calculators.ComputedSnapshot, int, int, error) {
	switch kpi {
	case constants.KpiOSA:
		params := calculators.ParseOSAParams(globalParams(resolver, kpi))
		// Окно [as_of - lookback_days, as_of - 1]
		from := in.AsOfDate.AddDate(0, 0, -params.LookbackDays)
		to := in.AsOfDate.AddDate(0, 0, -1)
		rows, err := e.repo.ReadDemandHistoryAggregates(ctx, from, to)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("read demand history: %w", err)
		}
		snaps, errs := calculators.ComputeOSA(rows, params)
		return snaps, errs, len(rows), nil

	case constants.KpiOTIF:
		params := calculators.ParseOTIFParams(globalParams(resolver, kpi))
		// Окно — последние 4 недели.
		from := in.AsOfDate.AddDate(0, 0, -28) //nolint:mnd // 4 weeks
		to := in.AsOfDate
		rows, err := e.repo.ReadSupplierScorecard(ctx, from, to)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("read scorecard: %w", err)
		}
		snaps, errs := calculators.ComputeOTIF(rows, params)
		return snaps, errs, len(rows), nil

	case constants.KpiStockDays:
		params := calculators.ParseStockDaysParams(globalParams(resolver, kpi))
		rows, err := e.repo.ReadCalculationInput(ctx)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("read calc input: %w", err)
		}
		snaps, errs := calculators.ComputeStockDays(rows, params)
		return snaps, errs, len(rows), nil

	default:
		return nil, 0, 0, fmt.Errorf("kpi engine: unknown kpi %q", kpi)
	}
}

// globalParams — берёт params из global-калибровки KPI или {} если её нет.
func globalParams(resolver *calibration.Resolver, kpi string) json.RawMessage {
	c := resolver.Resolve(kpi, calibration.ScopeKeys{})
	if len(c.Params) == 0 {
		return json.RawMessage(`{}`)
	}
	return c.Params
}

// matchedCalibrationID — детектит, какой scope-калибрatation матчит снапшот.
//
// Для product_location пытается найти specific калибровку; для supplier — supplier-level;
// иначе fallback на global.
func matchedCalibrationID(resolver *calibration.Resolver, s calculators.ComputedSnapshot) uuid.UUID {
	keys := calibration.ScopeKeys{}
	if s.ScopeType == constants.ScopeTypeProductLocation && s.ScopeID != nil {
		keys.ProductLocation = s.ScopeID
	}
	if s.ScopeType == constants.ScopeTypeSupplier && s.ScopeID != nil {
		keys.SupplierID = s.ScopeID
	}
	if s.ScopeType == constants.ScopeTypeLocation && s.ScopeID != nil {
		keys.LocationID = s.ScopeID
	}
	c := resolver.Resolve(s.KpiName, keys)
	return c.ID
}
