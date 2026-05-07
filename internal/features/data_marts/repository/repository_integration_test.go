//go:build integration
// +build integration

package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/repository"
)

// --- dockertest fixture (postgres:18-alpine) ---

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
				"listen_addresses=*",
			},
		}, func(c *docker.HostConfig) {
			c.AutoRemove = true
			c.RestartPolicy = docker.RestartPolicy{Name: "no"}
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

		// Применяем migration 1001_marts_schema.up.sql напрямую (без golang-migrate).
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			fixError = err
			_ = dpool.Purge(resource)
			return
		}
		// Минимальная схема для теста (cтабильнее, чем тянуть golang-migrate).
		if _, err := pool.Exec(ctx, `
			CREATE SCHEMA IF NOT EXISTS marts;
			CREATE TABLE IF NOT EXISTS marts.etl_runs (
				id UUID PRIMARY KEY,
				started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				finished_at TIMESTAMPTZ,
				committed_at TIMESTAMPTZ,
				status TEXT NOT NULL,
				kind TEXT NOT NULL DEFAULT 'full',
				target_mart TEXT,
				source_load_id UUID,
				parent_run_id UUID,
				trigger TEXT NOT NULL DEFAULT 'cron',
				requester TEXT,
				marts_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
				failure_reason TEXT,
				lines_total BIGINT NOT NULL DEFAULT 0,
				lines_failed BIGINT NOT NULL DEFAULT 0,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			);
			CREATE TABLE IF NOT EXISTS marts.mart_demand_history (
				as_of_date DATE NOT NULL,
				location_id TEXT NOT NULL,
				product_id TEXT NOT NULL,
				qty_sold NUMERIC(18,4) NOT NULL DEFAULT 0,
				qty_returned NUMERIC(18,4) NOT NULL DEFAULT 0,
				qty_promo_bonus NUMERIC(18,4) NOT NULL DEFAULT 0,
				qty_gift NUMERIC(18,4) NOT NULL DEFAULT 0,
				revenue_paid NUMERIC(18,4) NOT NULL DEFAULT 0,
				discount_total NUMERIC(18,4) NOT NULL DEFAULT 0,
				transactions_count INTEGER NOT NULL DEFAULT 0,
				had_promo BOOLEAN NOT NULL DEFAULT false,
				promo_type TEXT,
				was_in_assortment BOOLEAN NOT NULL DEFAULT false,
				lifecycle_state_at_date TEXT,
				was_oos BOOLEAN NOT NULL DEFAULT false,
				etl_run_id UUID NOT NULL,
				source_load_id UUID NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				PRIMARY KEY (product_id, location_id, as_of_date)
			);
		`); err != nil {
			fixError = err
			pool.Close()
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
		t.Fatalf("dockertest fixture: %v", fixError)
	}
	return fix
}

// TestRepository_GetCurrentVersion_NoCommittedRun_ServiceUnavailable.
func TestRepository_GetCurrentVersion_NoCommittedRun_ServiceUnavailable(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	t.Cleanup(func() {})

	ctx := context.Background()
	// Truncate чтобы изоляция теста.
	_, err := f.pool.Exec(ctx, "TRUNCATE TABLE marts.etl_runs CASCADE")
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, "TRUNCATE TABLE marts.mart_demand_history")
	require.NoError(t, err)

	repo := repository.New(f.pool)
	_, err = repo.GetCurrentVersion(ctx)
	require.Error(t, err, "пустые etl_runs → ErrServiceUnavailable")
}

// TestRepository_SelectDemandHistory_HappyPath.
func TestRepository_SelectDemandHistory_HappyPath(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)

	ctx := context.Background()
	_, err := f.pool.Exec(ctx, "TRUNCATE TABLE marts.etl_runs CASCADE")
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, "TRUNCATE TABLE marts.mart_demand_history")
	require.NoError(t, err)

	runID := uuid.New()
	srcID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	_, err = f.pool.Exec(ctx, `
		INSERT INTO marts.etl_runs (id, started_at, finished_at, committed_at, status, trigger)
		VALUES ($1, $2, $2, $2, 'committed', 'cron')`, runID, now)
	require.NoError(t, err)

	_, err = f.pool.Exec(ctx, `
		INSERT INTO marts.mart_demand_history (
			as_of_date, location_id, product_id,
			qty_sold, qty_returned, qty_promo_bonus, qty_gift,
			revenue_paid, discount_total, transactions_count,
			had_promo, was_in_assortment, was_oos,
			etl_run_id, source_load_id
		) VALUES ('2026-05-01', 'L1', 'P1', 10, 0, 0, 0, 100, 0, 1, false, true, false, $1, $2)`,
		runID, srcID)
	require.NoError(t, err)

	repo := repository.New(f.pool)

	v, err := repo.GetCurrentVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, runID, v.EtlRunID)

	rows, nextPK, err := repo.SelectMartRows(ctx, constants.MartDemandHistory, runID, models.Cursor{}, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "P1", rows[0]["product_id"])
	require.Equal(t, "L1", rows[0]["location_id"])
	require.Empty(t, nextPK, "одна строка < limit → nextPK пуст")
}
