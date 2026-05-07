package loader_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/loader"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/transformer"
)

// --- BuildSummary ---

func TestBuildSummary_AddTotal(t *testing.T) {
	t.Parallel()
	s := loader.NewBuildSummary()
	s.Add("a", 10)
	s.Add("b", 20)
	s.Add("", 5) // ignored
	assert.Equal(t, int64(30), s.Total())
}

func TestBuildSummary_MarshalJSONB(t *testing.T) {
	t.Parallel()
	s := loader.NewBuildSummary()
	s.Add("mart_demand_history", 100)
	s.Add("mart_kpi_daily", 25)
	raw, err := s.MarshalJSONB()
	require.NoError(t, err)
	var decoded map[string]map[string]int64
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, int64(100), decoded["mart_demand_history"]["rows"])
	assert.Equal(t, int64(25), decoded["mart_kpi_daily"]["rows"])
}

// --- fake pgx.Tx implementation, with the minimum surface that loader uses ---

type fakeTx struct {
	pgx.Tx // embeds nil — Loader doesn't call methods on tx directly; only passes it to Builders / repo.
	pool   *fakePool
}

func (t *fakeTx) Commit(_ context.Context) error {
	if t.pool.commitErr != nil {
		return t.pool.commitErr
	}
	t.pool.committed = true
	return nil
}

func (t *fakeTx) Rollback(_ context.Context) error {
	if t.pool.committed {
		// pgx behavior: rollback after commit is a no-op (returns ErrTxClosed),
		// we simulate it by not flipping rolled.
		return pgx.ErrTxClosed
	}
	t.pool.rolled = true
	return nil
}

type fakePool struct {
	beginErr  error
	commitErr error
	committed bool
	rolled    bool
}

func (p *fakePool) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	if p.beginErr != nil {
		return nil, p.beginErr
	}
	return &fakeTx{pool: p}, nil
}

// --- fake repo (EtlRunsUpdater) ---

type fakeRepo struct {
	stagingErr   error
	updateErr    error
	stagingCalls int
	updatedID    uuid.UUID
	updatedPatch repository.EtlRunStatusPatch
}

func (r *fakeRepo) CreateStagingTables(_ context.Context, _ pgx.Tx) error {
	r.stagingCalls++
	return r.stagingErr
}

func (r *fakeRepo) UpdateEtlRunStatusTx(_ context.Context, _ pgx.Tx, id uuid.UUID, p repository.EtlRunStatusPatch) error {
	r.updatedID = id
	r.updatedPatch = p
	return r.updateErr
}

// --- fake Builder ---

type fakeBuilder struct {
	name     string
	rows     int64
	buildErr error
	onDemand bool
	calls    int
}

func (b *fakeBuilder) Name() string       { return b.name }
func (b *fakeBuilder) OnDemandOnly() bool { return b.onDemand }
func (b *fakeBuilder) Build(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (int64, error) {
	b.calls++
	return b.rows, b.buildErr
}

func newApplyParams(builders []transformer.Builder) loader.ApplyParams {
	return loader.ApplyParams{
		RunID:        uuid.New(),
		SourceLoadID: uuid.New(),
		Builders:     builders,
		LinesTotal:   100,
		LinesFailed:  2,
	}
}

func TestLoader_Apply_HappyPath(t *testing.T) {
	t.Parallel()
	pool := &fakePool{}
	repo := &fakeRepo{}
	l := loader.New(pool, repo, nil)
	b1 := &fakeBuilder{name: constants.MartMasterCurrent, rows: 10}
	b2 := &fakeBuilder{name: constants.MartDemandHistory, rows: 100}
	params := newApplyParams([]transformer.Builder{b1, b2})

	summary, err := l.Apply(context.Background(), params)
	require.NoError(t, err)
	assert.True(t, pool.committed)
	assert.False(t, pool.rolled)
	assert.Equal(t, int64(110), summary.Total())
	assert.Equal(t, 1, repo.stagingCalls)
	assert.Equal(t, params.RunID, repo.updatedID)
	assert.Equal(t, constants.StatusCommitted, repo.updatedPatch.Status)
	assert.NotNil(t, repo.updatedPatch.CommittedAt)
	assert.NotNil(t, repo.updatedPatch.FinishedAt)
	assert.Equal(t, 1, b1.calls)
	assert.Equal(t, 1, b2.calls)
}

func TestLoader_Apply_BuilderError_Rollback(t *testing.T) {
	t.Parallel()
	pool := &fakePool{}
	repo := &fakeRepo{}
	l := loader.New(pool, repo, nil)
	b1 := &fakeBuilder{name: "m1", rows: 5}
	b2 := &fakeBuilder{name: "m2", buildErr: errors.New("boom")}
	params := newApplyParams([]transformer.Builder{b1, b2})

	_, err := l.Apply(context.Background(), params)
	require.Error(t, err)
	assert.False(t, pool.committed)
	assert.True(t, pool.rolled)
	assert.Equal(t, 1, b1.calls)
	assert.Equal(t, 1, b2.calls)
}

func TestLoader_Apply_StagingError(t *testing.T) {
	t.Parallel()
	pool := &fakePool{}
	repo := &fakeRepo{stagingErr: errors.New("staging fail")}
	l := loader.New(pool, repo, nil)
	params := newApplyParams([]transformer.Builder{&fakeBuilder{name: "m1", rows: 1}})
	_, err := l.Apply(context.Background(), params)
	require.Error(t, err)
	assert.True(t, pool.rolled)
}

func TestLoader_Apply_BeginError(t *testing.T) {
	t.Parallel()
	pool := &fakePool{beginErr: errors.New("conn dropped")}
	l := loader.New(pool, &fakeRepo{}, nil)
	params := newApplyParams([]transformer.Builder{&fakeBuilder{name: "m"}})
	_, err := l.Apply(context.Background(), params)
	require.Error(t, err)
}

func TestLoader_Apply_CommitError(t *testing.T) {
	t.Parallel()
	pool := &fakePool{commitErr: errors.New("commit fail")}
	l := loader.New(pool, &fakeRepo{}, nil)
	params := newApplyParams([]transformer.Builder{&fakeBuilder{name: "m", rows: 1}})
	_, err := l.Apply(context.Background(), params)
	require.Error(t, err)
}

func TestLoader_Apply_UpdateError_Rollback(t *testing.T) {
	t.Parallel()
	pool := &fakePool{}
	repo := &fakeRepo{updateErr: errors.New("etl_runs update fail")}
	l := loader.New(pool, repo, nil)
	params := newApplyParams([]transformer.Builder{&fakeBuilder{name: "m", rows: 1}})
	_, err := l.Apply(context.Background(), params)
	require.Error(t, err)
	assert.True(t, pool.rolled)
}

func TestLoader_Apply_BadParams(t *testing.T) {
	t.Parallel()
	l := loader.New(&fakePool{}, &fakeRepo{}, nil)

	_, err := l.Apply(context.Background(), loader.ApplyParams{
		SourceLoadID: uuid.New(),
		Builders:     []transformer.Builder{&fakeBuilder{name: "m"}},
	})
	require.Error(t, err, "RunID nil")

	_, err = l.Apply(context.Background(), loader.ApplyParams{
		RunID:    uuid.New(),
		Builders: []transformer.Builder{&fakeBuilder{name: "m"}},
	})
	require.Error(t, err, "SourceLoadID nil")

	_, err = l.Apply(context.Background(), loader.ApplyParams{
		RunID:        uuid.New(),
		SourceLoadID: uuid.New(),
	})
	require.Error(t, err, "Builders empty")
}

func TestLoader_AssertPoolOK(t *testing.T) {
	t.Parallel()
	require.Error(t, loader.AssertPoolOK(nil))
}
