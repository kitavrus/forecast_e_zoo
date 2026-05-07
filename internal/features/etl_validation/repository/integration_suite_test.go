//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

// pgFixture — общий dockertest-контейнер на пакет repository_test.
type pgFixture struct {
	pool     *pgxpool.Pool
	dsn      string
	tearDown func()
}

var (
	fixOnce  sync.Once
	fix      *pgFixture
	fixError error
)

func skipIfDocker(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test")
	}
	if os.Getenv("SKIP_DOCKER") == "1" {
		t.Skip("SKIP_DOCKER=1")
	}
}

// migrationsDir → ../sqls/migrations.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(here), "..", "sqls", "migrations")
}

func setupFixture(t *testing.T) *pgFixture {
	t.Helper()
	fixOnce.Do(func() {
		dpool, err := dockertest.NewPool("")
		if err != nil {
			fixError = err
			return
		}
		if err := dpool.Client.Ping(); err != nil {
			fixError = err
			return
		}
		resource, err := dpool.RunWithOptions(&dockertest.RunOptions{
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
		if err != nil {
			fixError = err
			return
		}
		hostPort := resource.GetHostPort("5432/tcp")
		dsn := fmt.Sprintf("postgres://test:test@%s/test?sslmode=disable", hostPort)

		dpool.MaxWait = 90 * time.Second
		if err := dpool.Retry(func() error {
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			return db.PingContext(ctx)
		}); err != nil {
			fixError = err
			_ = dpool.Purge(resource)
			return
		}

		dir := migrationsDir(t)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			fixError = err
			_ = dpool.Purge(resource)
			return
		}
		drv, err := migratepg.WithInstance(db, &migratepg.Config{})
		if err != nil {
			_ = db.Close()
			fixError = err
			_ = dpool.Purge(resource)
			return
		}
		m, err := migrate.NewWithDatabaseInstance("file://"+dir, "postgres", drv)
		if err != nil {
			_ = db.Close()
			fixError = err
			_ = dpool.Purge(resource)
			return
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			_ = db.Close()
			fixError = err
			_ = dpool.Purge(resource)
			return
		}
		_ = db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			fixError = err
			_ = dpool.Purge(resource)
			return
		}
		fix = &pgFixture{
			pool: pool,
			dsn:  dsn,
			tearDown: func() {
				pool.Close()
				_ = dpool.Purge(resource)
			},
		}
	})
	if fixError != nil {
		t.Skipf("docker fixture not available: %v", fixError)
	}
	return fix
}

// truncateAll — изоляция между тестами: чистим etl_runs / reject_log / audit_access.
func truncateAll(t *testing.T, ctx context.Context) {
	t.Helper()
	_, err := fix.pool.Exec(ctx, `TRUNCATE TABLE marts.reject_log, marts.audit_access RESTART IDENTITY;`)
	require.NoError(t, err)
	_, err = fix.pool.Exec(ctx, `DELETE FROM marts.etl_runs;`)
	require.NoError(t, err)
}

// TestMain — единая точка teardown.
func TestMain(m *testing.M) {
	code := m.Run()
	if fix != nil {
		fix.tearDown()
	}
	os.Exit(code)
}
