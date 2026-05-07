package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/extractor"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/loader"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/transformer"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
)

// --- fakes ---

type fakeRepo struct {
	mu             sync.Mutex
	updates        []repository.EtlRunStatusPatch
	insertedReject []models.RejectLogEntry
}

func (f *fakeRepo) InsertEtlRun(_ context.Context, _ *models.EtlRun) error { return nil }
func (f *fakeRepo) GetEtlRunByID(_ context.Context, _ uuid.UUID) (*models.EtlRun, error) {
	return nil, nil
}
func (f *fakeRepo) ListEtlRuns(_ context.Context, _ repository.EtlRunListFilter) ([]models.EtlRun, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateEtlRunStatus(_ context.Context, _ uuid.UUID, p repository.EtlRunStatusPatch) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, p)
	return nil
}
func (f *fakeRepo) UpdateEtlRunStatusTx(_ context.Context, _ pgx.Tx, _ uuid.UUID, p repository.EtlRunStatusPatch) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, p)
	return nil
}
func (f *fakeRepo) GetCurrentRunningEtlRun(_ context.Context) (*models.EtlRun, error) {
	return nil, nil
}
func (f *fakeRepo) InsertRejectEntries(_ context.Context, entries []models.RejectLogEntry) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertedReject = append(f.insertedReject, entries...)
	return int64(len(entries)), nil
}
func (f *fakeRepo) ListRejectEntries(_ context.Context, _ repository.RejectLogListFilter) ([]models.RejectLogEntry, error) {
	return nil, nil
}
func (f *fakeRepo) CreateStagingTables(_ context.Context, _ pgx.Tx) error { return nil }
func (f *fakeRepo) TryAdvisoryXactLock(_ context.Context, _ pgx.Tx, _ int64) (bool, error) {
	return true, nil
}

func (f *fakeRepo) lastUpdate() *repository.EtlRunStatusPatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.updates) == 0 {
		return nil
	}
	cp := f.updates[len(f.updates)-1]
	return &cp
}

// fakeNDJSON — пустой NDJSONReader; всегда EOF.
type fakeNDJSON struct{}

func (fakeNDJSON) Next(_ any) error { return io.EOF }
func (fakeNDJSON) ETag() string     { return "" }
func (fakeNDJSON) Close() error     { return nil }

type fakeExtractor struct {
	loadID  string
	snapErr error
}

func (e fakeExtractor) GetCurrentSnapshot(_ context.Context) (extractor.Snapshot, error) {
	if e.snapErr != nil {
		return extractor.Snapshot{}, e.snapErr
	}
	return extractor.Snapshot{CurrentLoadID: e.loadID, CommittedAt: time.Now()}, nil
}
func (e fakeExtractor) StreamEntity(_ context.Context, _, _, _ string, _, _ time.Time) (extractor.NDJSONReader, error) {
	return fakeNDJSON{}, nil
}

type fakeEngine struct {
	report validation.Report
}

func (f fakeEngine) Run(_ *validation.Dataset) validation.Report { return f.report }

type fakeLoader struct {
	calls   int
	gotPop  bool
	rows    int64
	mart    string
	failErr error
}

func (l *fakeLoader) Apply(_ context.Context, p loader.ApplyParams) (loader.BuildSummary, error) {
	l.calls++
	if p.PopulateStaging != nil {
		l.gotPop = true
	}
	if l.failErr != nil {
		return nil, l.failErr
	}
	s := loader.NewBuildSummary()
	if l.mart != "" {
		s.Add(l.mart, l.rows)
	}
	return s, nil
}

type fakeRegistry struct {
	full []transformer.Builder
}

func (r fakeRegistry) BuildersForFullRun() []transformer.Builder { return r.full }
func (r fakeRegistry) BuilderByName(_ string) (transformer.Builder, error) {
	return nil, errors.New("not implemented")
}

type fakeBuilder struct{ name string }

func (b fakeBuilder) Name() string                                              { return b.name }
func (b fakeBuilder) OnDemandOnly() bool                                        { return false }
func (b fakeBuilder) Build(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (int64, error) {
	return 0, nil
}

type fakeMetrics struct {
	successCalled int
	failureReason string
	rowsByMart    map[string]int64
}

func (f *fakeMetrics) RecordRunSuccess(_ float64)              { f.successCalled++ }
func (f *fakeMetrics) RecordRunFailure(_ float64, reason string) { f.failureReason = reason }
func (f *fakeMetrics) RecordRowsProcessed(mart string, rows int64) {
	if f.rowsByMart == nil {
		f.rowsByMart = make(map[string]int64)
	}
	f.rowsByMart[mart] = rows
}

type fakePool struct{}

func (fakePool) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("not used in runAsync tests")
}

// --- runAsync happy path ---

func TestRunAsync_HappyPath_LoaderInvokedWithPopulateStaging(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	loadID := uuid.New().String()
	extr := fakeExtractor{loadID: loadID}
	eng := fakeEngine{report: validation.Report{LinesTotal: 0, LinesFailed: 0}}
	ld := &fakeLoader{mart: constants.MartMasterCurrent, rows: 10}
	reg := fakeRegistry{full: []transformer.Builder{fakeBuilder{name: constants.MartMasterCurrent}}}
	met := &fakeMetrics{}
	p := NewEtlPipeline(fakePool{}, repo, extr, eng, reg, ld, met, nil, EtlPipelineConfig{
		QualityThreshold: 0.01,
		RunTimeout:       5 * time.Second,
	})

	runID := uuid.New()
	p.runAsync(runID)

	assert.Equal(t, 1, ld.calls, "loader.Apply must be invoked once")
	assert.True(t, ld.gotPop, "loader.Apply must receive PopulateStaging callback")
	assert.Equal(t, 1, met.successCalled)
	assert.Empty(t, met.failureReason)

	// Должен быть как минимум один update с source_load_id (best-effort persist).
	hasSourceLoadID := false
	repo.mu.Lock()
	for _, u := range repo.updates {
		if u.SourceLoadID != nil {
			hasSourceLoadID = true
		}
	}
	repo.mu.Unlock()
	assert.True(t, hasSourceLoadID, "source_load_id must be persisted")
}

func TestRunAsync_QualityThreshold_MarksFailed(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	extr := fakeExtractor{loadID: uuid.New().String()}
	eng := fakeEngine{report: validation.Report{LinesTotal: 100, LinesFailed: 50}} // 50% > 1%
	ld := &fakeLoader{}
	reg := fakeRegistry{full: []transformer.Builder{fakeBuilder{name: constants.MartMasterCurrent}}}
	met := &fakeMetrics{}
	p := NewEtlPipeline(fakePool{}, repo, extr, eng, reg, ld, met, nil, EtlPipelineConfig{
		QualityThreshold: 0.01,
		RunTimeout:       5 * time.Second,
	})

	p.runAsync(uuid.New())

	assert.Equal(t, 0, ld.calls, "loader must NOT be called on quality fail")
	assert.Equal(t, "quality_threshold", met.failureReason)
	last := repo.lastUpdate()
	require.NotNil(t, last)
	assert.Equal(t, constants.StatusFailed, last.Status)
	require.NotNil(t, last.FailureReason)
	assert.Contains(t, *last.FailureReason, "quality threshold")
}

func TestRunAsync_BadSnapshotID_MarksFailed(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	extr := fakeExtractor{loadID: "not-a-uuid"}
	met := &fakeMetrics{}
	p := NewEtlPipeline(fakePool{}, repo, extr, fakeEngine{}, fakeRegistry{}, &fakeLoader{}, met, nil, EtlPipelineConfig{
		RunTimeout: 5 * time.Second,
	})

	p.runAsync(uuid.New())

	assert.Equal(t, "bad_source_load_id", met.failureReason)
	last := repo.lastUpdate()
	require.NotNil(t, last)
	assert.Equal(t, constants.StatusFailed, last.Status)
}

func TestRunAsync_SnapshotError_MarksFailed(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	extr := fakeExtractor{snapErr: errors.New("source down")}
	met := &fakeMetrics{}
	p := NewEtlPipeline(fakePool{}, repo, extr, fakeEngine{}, fakeRegistry{}, &fakeLoader{}, met, nil, EtlPipelineConfig{
		RunTimeout: 5 * time.Second,
	})

	p.runAsync(uuid.New())

	assert.Equal(t, "snapshot", met.failureReason)
	last := repo.lastUpdate()
	require.NotNil(t, last)
	assert.Equal(t, constants.StatusFailed, last.Status)
}

func TestRunAsync_LoaderError_MarksFailed(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	extr := fakeExtractor{loadID: uuid.New().String()}
	ld := &fakeLoader{failErr: errors.New("commit dropped")}
	reg := fakeRegistry{full: []transformer.Builder{fakeBuilder{name: constants.MartMasterCurrent}}}
	met := &fakeMetrics{}
	p := NewEtlPipeline(fakePool{}, repo, extr, fakeEngine{}, reg, ld, met, nil, EtlPipelineConfig{
		RunTimeout: 5 * time.Second,
	})

	p.runAsync(uuid.New())

	assert.Equal(t, "loader", met.failureReason)
	last := repo.lastUpdate()
	require.NotNil(t, last)
	assert.Equal(t, constants.StatusFailed, last.Status)
	require.NotNil(t, last.FailureReason)
	assert.True(t, strings.Contains(*last.FailureReason, "loader"))
}

func TestRunAsync_ViolationsPersisted(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	extr := fakeExtractor{loadID: uuid.New().String()}
	eng := fakeEngine{report: validation.Report{
		LinesTotal:  100,
		LinesFailed: 0,
		Violations: []validation.Violation{
			{RuleName: "r1", Entity: "product", Severity: validation.SeveritySoft, Message: "x"},
		},
	}}
	ld := &fakeLoader{mart: constants.MartMasterCurrent, rows: 10}
	reg := fakeRegistry{full: []transformer.Builder{fakeBuilder{name: constants.MartMasterCurrent}}}
	met := &fakeMetrics{}
	p := NewEtlPipeline(fakePool{}, repo, extr, eng, reg, ld, met, nil, EtlPipelineConfig{
		QualityThreshold: 0.5,
		RunTimeout:       5 * time.Second,
	})

	p.runAsync(uuid.New())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.insertedReject, 1)
	assert.Equal(t, "product", repo.insertedReject[0].Entity)
	assert.Equal(t, "soft", repo.insertedReject[0].Severity)
}
