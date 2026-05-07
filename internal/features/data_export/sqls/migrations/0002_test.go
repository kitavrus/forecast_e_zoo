//go:build integration
// +build integration

package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var partitionedFacts = []string{
	"receipt_line",
	"location_stock_snapshot",
	"stock_movement",
	"supplier_stock_snapshot",
}

func TestMigration0002_PartitionedTablesExist(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(context.Background(), `
		SELECT c.relname
		FROM pg_partitioned_table pt
		JOIN pg_class c ON c.oid = pt.partrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	got := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		got[name] = true
	}
	require.NoError(t, rows.Err())

	for _, name := range partitionedFacts {
		assert.Truef(t, got[name], "expected partitioned table %q", name)
	}
}

func TestMigration0002_InitialPartitionsCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	for _, name := range partitionedFacts {
		var cnt int
		require.NoError(t, db.QueryRow(`
			SELECT count(*)
			FROM pg_inherits i
			JOIN pg_class p ON p.oid = i.inhparent
			WHERE p.relname = $1`, name).Scan(&cnt))
		assert.GreaterOrEqualf(t, cnt, 4, "expected >=4 partitions for %q", name)
	}
}

func TestMigration0002_InsertRoutesToCorrectPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		INSERT INTO receipt_line (id, receipt_id, location_id, product_id, qty, price, event_time, event_date)
		VALUES (1, 'r-1', 'loc-1', 'prod-1', 2.0, 100.0, '2026-05-15 12:00:00+00', '2026-05-15')`)
	require.NoError(t, err)

	var partition string
	require.NoError(t, db.QueryRow(`
		SELECT tableoid::regclass::text
		FROM receipt_line
		WHERE id = 1 AND event_date = '2026-05-15'`).Scan(&partition))
	assert.Equal(t, "receipt_line_y2026m05", partition)
}

func TestMigration0002_InsertOutsideRange_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`
		INSERT INTO receipt_line (id, receipt_id, location_id, product_id, qty, price, event_time, event_date)
		VALUES (1, 'r-1', 'loc-1', 'prod-1', 2.0, 100.0, '2030-01-01 12:00:00+00', '2030-01-01')`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no partition of relation")
}
