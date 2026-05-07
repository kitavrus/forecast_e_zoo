package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
)

func TestRowSource_HappyPath(t *testing.T) {
	t.Parallel()
	cols := []string{"id", "name", "is_active"}
	rows := []validation.Row{
		{"id": "p1", "name": "Apple", "is_active": true},
		{"id": "p2", "name": "Banana", "is_active": false},
	}
	src := newRowSource(cols, rows)

	require.True(t, src.Next())
	v, err := src.Values()
	require.NoError(t, err)
	assert.Equal(t, []any{"p1", "Apple", true}, v)

	require.True(t, src.Next())
	v, err = src.Values()
	require.NoError(t, err)
	assert.Equal(t, []any{"p2", "Banana", false}, v)

	require.False(t, src.Next())
	require.NoError(t, src.Err())
}

func TestRowSource_NilAndMissing(t *testing.T) {
	t.Parallel()
	cols := []string{"id", "name", "is_active"}
	rows := []validation.Row{
		{"id": "p1"},                              // name + is_active отсутствуют
		{"id": "p2", "name": nil, "is_active": nil}, // явный nil
	}
	src := newRowSource(cols, rows)

	require.True(t, src.Next())
	v, err := src.Values()
	require.NoError(t, err)
	assert.Equal(t, []any{"p1", nil, nil}, v)

	require.True(t, src.Next())
	v, err = src.Values()
	require.NoError(t, err)
	assert.Equal(t, []any{"p2", nil, nil}, v)
}

func TestRowSource_DateConversion(t *testing.T) {
	t.Parallel()
	cols := []string{"product_id", "valid_from", "valid_to"}
	rows := []validation.Row{
		{"product_id": "p1", "valid_from": "2026-01-01", "valid_to": "2026-12-31"},
	}
	src := newRowSource(cols, rows)
	require.True(t, src.Next())
	v, err := src.Values()
	require.NoError(t, err)
	assert.Equal(t, "p1", v[0])
	tm0, ok := v[1].(time.Time)
	require.True(t, ok, "valid_from should be time.Time, got %T", v[1])
	assert.Equal(t, 2026, tm0.Year())
	tm1, ok := v[2].(time.Time)
	require.True(t, ok, "valid_to should be time.Time")
	assert.Equal(t, time.December, tm1.Month())
}

func TestRowSource_TimestampConversion(t *testing.T) {
	t.Parallel()
	cols := []string{"event_time"}
	rows := []validation.Row{
		{"event_time": "2026-05-07T12:30:00Z"},
	}
	src := newRowSource(cols, rows)
	require.True(t, src.Next())
	v, err := src.Values()
	require.NoError(t, err)
	tm, ok := v[0].(time.Time)
	require.True(t, ok)
	assert.Equal(t, 2026, tm.Year())
	assert.Equal(t, 12, tm.Hour())
}

func TestRowSource_BadDate(t *testing.T) {
	t.Parallel()
	cols := []string{"valid_from"}
	rows := []validation.Row{{"valid_from": "not-a-date"}}
	src := newRowSource(cols, rows)
	require.True(t, src.Next())
	_, err := src.Values()
	require.Error(t, err)
}

func TestStagingByEntity_AllStagedEntitiesAreAllowed(t *testing.T) {
	t.Parallel()
	// Контракт: каждая entity, имеющая staging-спеку, должна присутствовать в
	// constants.AllowedEntities (имена строго соответствуют путям source-adapter).
	// Обратное НЕ требуется — entity без mart-логики (product_barcodes, category,
	// supply_plan, master_change_log, store_assortment_lifecycle_events,
	// stock_movement, supplier_stock_snapshot) экстрактятся, но не загружаются
	// в pg_temp.stg_*.
	for entity := range stagingByEntity {
		assert.Contains(t, constants.AllowedEntities, entity,
			"entity %q in stagingByEntity must be in constants.AllowedEntities", entity)
	}
}

func TestStagingByEntity_HasMandatoryFields(t *testing.T) {
	t.Parallel()
	for entity, spec := range stagingByEntity {
		assert.NotEmpty(t, spec.table, "entity %q: empty staging table", entity)
		assert.NotEmpty(t, spec.columns, "entity %q: empty columns", entity)
	}
}

func TestPopulateStaging_EmptyRowsByEnt(t *testing.T) {
	t.Parallel()
	// nil callback не должен паниковать; пустой map → no-op.
	cb := populateStaging(nil)
	require.NotNil(t, cb)
	cb = populateStaging(map[string][]validation.Row{})
	require.NotNil(t, cb)
}

func TestParseDate_Variants(t *testing.T) {
	t.Parallel()
	v, err := parseDate("2026-05-07")
	require.NoError(t, err)
	tm, ok := v.(time.Time)
	require.True(t, ok)
	assert.Equal(t, 2026, tm.Year())

	v, err = parseDate("2026-05-07T12:00:00Z")
	require.NoError(t, err)
	_, ok = v.(time.Time)
	require.True(t, ok)

	v, err = parseDate("")
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = parseDate(time.Now())
	require.NoError(t, err)
	_, ok = v.(time.Time)
	require.True(t, ok)

	_, err = parseDate(123)
	require.Error(t, err)
}

func TestParseTimestamp_Variants(t *testing.T) {
	t.Parallel()
	v, err := parseTimestamp("2026-05-07T12:00:00Z")
	require.NoError(t, err)
	_, ok := v.(time.Time)
	require.True(t, ok)

	v, err = parseTimestamp("")
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = parseTimestamp(time.Now())
	require.NoError(t, err)
	_, ok = v.(time.Time)
	require.True(t, ok)

	v, err = parseTimestamp(float64(1700000000))
	require.NoError(t, err)
	_, ok = v.(time.Time)
	require.True(t, ok)

	_, err = parseTimestamp("garbage")
	require.Error(t, err)
}
