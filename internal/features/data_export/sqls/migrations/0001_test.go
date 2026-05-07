//go:build integration
// +build integration

package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedTables — ровно 17 таблиц, которые создаёт 0001_master_and_service.up.sql.
var expectedTables = []string{
	"loads",
	"snapshot_pointer",
	"reject_log",
	"entity_checkpoint",
	"audit_access",
	"category",
	"location",
	"supplier",
	"products",
	"product_barcodes",
	"store_assortment",
	"store_assortment_lifecycle_events",
	"supply_spec",
	"promo",
	"order_rule",
	"supply_plan",
	"master_change_log",
}

// migrationsDir возвращает абсолютный путь к директории с миграциями.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Dir(here)
}

// startPostgres поднимает postgres:18-alpine через dockertest и возвращает (DSN, cleanup).
func startPostgres(t *testing.T) (string, func()) {
	t.Helper()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)
	require.NoError(t, pool.Client.Ping())

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "18-alpine",
		Env: []string{
			"POSTGRES_USER=test",
			"POSTGRES_PASSWORD=test",
			"POSTGRES_DB=test",
		},
	}, func(cfg *docker.HostConfig) {
		cfg.AutoRemove = true
		cfg.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	require.NoError(t, err)

	hostPort := resource.GetHostPort("5432/tcp")
	dsn := fmt.Sprintf("postgres://test:test@%s/test?sslmode=disable", hostPort)

	pool.MaxWait = 60 * time.Second
	require.NoError(t, pool.Retry(func() error {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return db.PingContext(ctx)
	}))

	cleanup := func() {
		_ = pool.Purge(resource)
	}
	return dsn, cleanup
}

// newMigrate создаёт *migrate.Migrate, читая миграции с диска (file:// source).
func newMigrate(t *testing.T, dsn string) *migrate.Migrate {
	t.Helper()

	dir := migrationsDir(t)
	srcURL := "file://" + dir

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	driver, err := migratepg.WithInstance(db, &migratepg.Config{})
	require.NoError(t, err)

	src, err := (&file.File{}).Open(srcURL)
	require.NoError(t, err)

	m, err := migrate.NewWithInstance("file", src, "postgres", driver)
	require.NoError(t, err)
	return m
}

func tableNames(t *testing.T, dsn string) map[string]bool {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(context.Background(),
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	out := map[string]bool{}
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		out[n] = true
	}
	require.NoError(t, rows.Err())
	return out
}

func TestMigration0001_Up_CreatesAllTables(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	if os.Getenv("SKIP_DOCKER") == "1" {
		t.Skip("SKIP_DOCKER=1")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())

	tables := tableNames(t, dsn)
	for _, name := range expectedTables {
		assert.Truef(t, tables[name], "expected table %q to exist", name)
	}
}

func TestMigration0001_Down_DropsAll(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	if os.Getenv("SKIP_DOCKER") == "1" {
		t.Skip("SKIP_DOCKER=1")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())
	require.NoError(t, m.Down())

	tables := tableNames(t, dsn)
	for _, name := range expectedTables {
		assert.Falsef(t, tables[name], "table %q must be dropped", name)
	}
}

func TestSnapshotPointerSingleRow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	if os.Getenv("SKIP_DOCKER") == "1" {
		t.Skip("SKIP_DOCKER=1")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var cnt int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM snapshot_pointer`).Scan(&cnt))
	assert.Equal(t, 1, cnt)

	// Вставка второй строки должна упасть на CHECK (id = 1).
	_, err = db.Exec(`INSERT INTO snapshot_pointer (id) VALUES (2)`)
	assert.Error(t, err)
}

func TestLoadsStatusCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	if os.Getenv("SKIP_DOCKER") == "1" {
		t.Skip("SKIP_DOCKER=1")
	}

	dsn, cleanup := startPostgres(t)
	defer cleanup()

	m := newMigrate(t, dsn)
	require.NoError(t, m.Up())

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		`INSERT INTO loads (load_id, source, status) VALUES ('00000000-0000-0000-0000-000000000001'::uuid, 'test', 'unknown')`,
	)
	assert.Error(t, err)

	_, err = db.Exec(
		`INSERT INTO loads (load_id, source, status) VALUES ('00000000-0000-0000-0000-000000000002'::uuid, 'test', 'running')`,
	)
	assert.NoError(t, err)
}
