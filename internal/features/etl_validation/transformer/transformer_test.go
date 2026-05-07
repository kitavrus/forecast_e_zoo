package transformer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/transformer"
)

// mockRepo — минимальная фейковая реализация MartUpserter для unit-тестов.
type mockRepo struct {
	rowsByCall map[string]int64
	errByCall  map[string]error
	calls      map[string]int
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		rowsByCall: map[string]int64{
			"demand": 100, "calc": 50, "kpi": 25, "master": 10, "scorecard": 5,
		},
		errByCall: map[string]error{},
		calls:     map[string]int{},
	}
}

func (m *mockRepo) UpsertDemandHistory(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (int64, error) {
	m.calls["demand"]++
	return m.rowsByCall["demand"], m.errByCall["demand"]
}

func (m *mockRepo) RebuildCalculationInput(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (int64, error) {
	m.calls["calc"]++
	return m.rowsByCall["calc"], m.errByCall["calc"]
}

func (m *mockRepo) UpsertKpiDaily(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (int64, error) {
	m.calls["kpi"]++
	return m.rowsByCall["kpi"], m.errByCall["kpi"]
}

func (m *mockRepo) RebuildMasterCurrent(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (int64, error) {
	m.calls["master"]++
	return m.rowsByCall["master"], m.errByCall["master"]
}

func (m *mockRepo) UpsertSupplierScorecard(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (int64, error) {
	m.calls["scorecard"]++
	return m.rowsByCall["scorecard"], m.errByCall["scorecard"]
}

// --- Builders ---

func TestBuilders_NamesAndOnDemand(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	cases := []struct {
		b      transformer.Builder
		name   string
		ondemand bool
	}{
		{transformer.NewDemandHistoryBuilder(repo), constants.MartDemandHistory, false},
		{transformer.NewCalculationInputBuilder(repo), constants.MartCalculationInput, false},
		{transformer.NewKpiDailyBuilder(repo), constants.MartKpiDaily, false},
		{transformer.NewMasterCurrentBuilder(repo), constants.MartMasterCurrent, false},
		{transformer.NewSupplierScorecardBuilder(repo), constants.MartSupplierScorecard, true},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.name, tc.b.Name())
		assert.Equal(t, tc.ondemand, tc.b.OnDemandOnly())
	}
}

func TestBuilders_BuildSuccess(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	ctx := context.Background()
	rid, sid := uuid.New(), uuid.New()

	builders := []transformer.Builder{
		transformer.NewDemandHistoryBuilder(repo),
		transformer.NewCalculationInputBuilder(repo),
		transformer.NewKpiDailyBuilder(repo),
		transformer.NewMasterCurrentBuilder(repo),
		transformer.NewSupplierScorecardBuilder(repo),
	}
	for _, b := range builders {
		n, err := b.Build(ctx, nil, rid, sid)
		require.NoError(t, err)
		assert.Greater(t, n, int64(0), "%s returned 0 rows", b.Name())
	}
}

func TestBuilders_BuildErrorWrapped(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	repo.errByCall["demand"] = errors.New("db boom")
	b := transformer.NewDemandHistoryBuilder(repo)
	_, err := b.Build(context.Background(), nil, uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), constants.MartDemandHistory)
	assert.Contains(t, err.Error(), "db boom")
}

// --- Registry ---

func TestRegistry_BuildersForFullRun_ExcludesOnDemand(t *testing.T) {
	t.Parallel()
	r := transformer.NewRegistry(newMockRepo())
	full := r.BuildersForFullRun()
	for _, b := range full {
		assert.False(t, b.OnDemandOnly(), "%s must not be in full run", b.Name())
		assert.NotEqual(t, constants.MartSupplierScorecard, b.Name())
	}
	// 4 mart-а в полном run.
	assert.Len(t, full, 4)
}

func TestRegistry_FullRunOrder_MasterFirst(t *testing.T) {
	t.Parallel()
	r := transformer.NewRegistry(newMockRepo())
	full := r.BuildersForFullRun()
	require.NotEmpty(t, full)
	// master_current — первый (он справочник, FK-цель остальных).
	assert.Equal(t, constants.MartMasterCurrent, full[0].Name())
}

func TestRegistry_BuilderByName(t *testing.T) {
	t.Parallel()
	r := transformer.NewRegistry(newMockRepo())
	b, err := r.BuilderByName(constants.MartSupplierScorecard)
	require.NoError(t, err)
	assert.Equal(t, constants.MartSupplierScorecard, b.Name())

	_, err = r.BuilderByName("nonexistent")
	require.Error(t, err)
}

func TestRegistry_Names(t *testing.T) {
	t.Parallel()
	r := transformer.NewRegistry(newMockRepo())
	names := r.Names()
	assert.Contains(t, names, constants.MartDemandHistory)
	assert.Contains(t, names, constants.MartSupplierScorecard)
	assert.Len(t, names, 5)
}
