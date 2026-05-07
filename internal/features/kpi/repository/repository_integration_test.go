//go:build integration
// +build integration

package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
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

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/repository"
)

type pgFixture struct {
	pool     *pgxpool.Pool
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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			fixError = err
			_ = dpool.Purge(resource)
			return
		}

		// Минимальная schema marts (для marts reader тестов) + KPI schema.
		if _, err := pool.Exec(ctx, `
			CREATE EXTENSION IF NOT EXISTS pgcrypto;
			CREATE SCHEMA IF NOT EXISTS marts;
			CREATE TABLE IF NOT EXISTS marts.mart_demand_history (
				as_of_date DATE NOT NULL,
				location_id TEXT NOT NULL,
				product_id TEXT NOT NULL,
				was_oos BOOLEAN NOT NULL DEFAULT false,
				etl_run_id UUID NOT NULL DEFAULT gen_random_uuid(),
				source_load_id UUID NOT NULL DEFAULT gen_random_uuid(),
				PRIMARY KEY (product_id, location_id, as_of_date)
			);
			CREATE TABLE IF NOT EXISTS marts.mart_calculation_input (
				product_id TEXT NOT NULL,
				location_id TEXT NOT NULL,
				on_hand NUMERIC(18,4) NOT NULL DEFAULT 0,
				in_transit NUMERIC(18,4) NOT NULL DEFAULT 0,
				daily_demand NUMERIC(18,4),
				supplier_id TEXT,
				applicable_rule_kind TEXT NOT NULL DEFAULT 'default',
				etl_run_id UUID NOT NULL DEFAULT gen_random_uuid(),
				source_load_id UUID NOT NULL DEFAULT gen_random_uuid(),
				PRIMARY KEY (product_id, location_id)
			);
			CREATE TABLE IF NOT EXISTS marts.mart_supplier_scorecard (
				supplier_id TEXT NOT NULL,
				week_start DATE NOT NULL,
				lines_delivered INTEGER NOT NULL DEFAULT 0,
				lines_late INTEGER NOT NULL DEFAULT 0,
				qty_short_total NUMERIC(18,4) NOT NULL DEFAULT 0,
				fill_rate_avg NUMERIC(8,4),
				etl_run_id UUID NOT NULL DEFAULT gen_random_uuid(),
				source_load_id UUID NOT NULL DEFAULT gen_random_uuid(),
				PRIMARY KEY (supplier_id, week_start)
			);
		`); err != nil {
			fixError = err
			pool.Close()
			_ = dpool.Purge(resource)
			return
		}

		// Apply KPI schema (Module 4, migration 2001).
		// Используем напрямую SQL вместо golang-migrate для стабильности тестов.
		if _, err := pool.Exec(ctx, `
			CREATE SCHEMA IF NOT EXISTS kpi;
			CREATE TABLE IF NOT EXISTS kpi.kpi_calibrations (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				kpi_name TEXT NOT NULL,
				scope_type TEXT NOT NULL CHECK (scope_type IN ('global','category','supplier','location','product_location')),
				scope_id TEXT,
				params JSONB NOT NULL DEFAULT '{}'::jsonb,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			);
			CREATE UNIQUE INDEX IF NOT EXISTS uq_kpi_calibrations_scope
				ON kpi.kpi_calibrations (kpi_name, scope_type, COALESCE(scope_id, ''));
			CREATE TABLE IF NOT EXISTS kpi.kpi_snapshots (
				id UUID NOT NULL DEFAULT gen_random_uuid(),
				as_of_date DATE NOT NULL,
				kpi_name TEXT NOT NULL,
				scope_type TEXT NOT NULL,
				scope_id TEXT,
				value NUMERIC(18,6) NOT NULL,
				calibration_id UUID,
				computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				etl_run_id UUID,
				created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
				PRIMARY KEY (id, as_of_date)
			) PARTITION BY RANGE (as_of_date);
			CREATE TABLE IF NOT EXISTS kpi.kpi_snapshots_default PARTITION OF kpi.kpi_snapshots DEFAULT;
			INSERT INTO kpi.kpi_calibrations (kpi_name, scope_type, scope_id, params) VALUES
				('osa', 'global', NULL, '{"lookback_days":30,"min_observations":7}'::jsonb),
				('otif', 'global', NULL, '{"late_grace_hours":0,"fill_rate_threshold":0.95}'::jsonb),
				('stock_days', 'global', NULL, '{"include_in_transit":true,"min_daily_demand":0.001,"cap_days":365}'::jsonb)
			ON CONFLICT DO NOTHING;
		`); err != nil {
			fixError = err
			pool.Close()
			_ = dpool.Purge(resource)
			return
		}

		fix = &pgFixture{
			pool: pool,
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

func cleanupSnapshots(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "TRUNCATE TABLE kpi.kpi_snapshots")
	require.NoError(t, err)
}

func TestRepository_InsertSnapshot_RoundTrip(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	cleanupSnapshots(t, f.pool)

	repo := repository.New(f.pool)
	ctx := context.Background()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	snap, err := repo.InsertSnapshot(ctx, repository.InsertSnapshotInput{
		AsOfDate:  asOf,
		KpiName:   constants.KpiOSA,
		ScopeType: constants.ScopeTypeGlobal,
		Value:     0.92,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, snap.ID)

	got, err := repo.GetSnapshotByID(ctx, snap.ID)
	require.NoError(t, err)
	require.Equal(t, constants.KpiOSA, got.KpiName)
	require.InDelta(t, 0.92, got.Value, 1e-6)
}

func TestRepository_GetSnapshotByID_NotFound(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	cleanupSnapshots(t, f.pool)

	repo := repository.New(f.pool)
	_, err := repo.GetSnapshotByID(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestRepository_ListSnapshots_FilterAndPaginate(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	cleanupSnapshots(t, f.pool)

	repo := repository.New(f.pool)
	ctx := context.Background()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		_, err := repo.InsertSnapshot(ctx, repository.InsertSnapshotInput{
			AsOfDate: asOf, KpiName: constants.KpiOSA,
			ScopeType: constants.ScopeTypeGlobal, Value: float64(i),
		})
		require.NoError(t, err)
	}

	kpi := constants.KpiOSA
	rows, _, err := repo.ListSnapshots(ctx, models.SnapshotFilter{KpiName: &kpi, Limit: 10})
	require.NoError(t, err)
	require.Len(t, rows, 3)
}

func TestRepository_ListCalibrations_HasSeeded(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)

	repo := repository.New(f.pool)
	ctx := context.Background()
	cs, err := repo.ListCalibrations(ctx, models.CalibrationFilter{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(cs), 3)
}

func TestRepository_UpdateCalibration_HappyPath(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)

	repo := repository.New(f.pool)
	ctx := context.Background()
	all, err := repo.ListCalibrations(ctx, models.CalibrationFilter{})
	require.NoError(t, err)
	require.NotEmpty(t, all)

	target := all[0]
	newParams := json.RawMessage(`{"lookback_days":14,"min_observations":3}`)
	updated, err := repo.UpdateCalibration(ctx, target.ID, newParams)
	require.NoError(t, err)
	require.JSONEq(t, string(newParams), string(updated.Params))
}

func TestRepository_DeleteSnapshotsForDate(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	cleanupSnapshots(t, f.pool)

	repo := repository.New(f.pool)
	ctx := context.Background()
	asOf := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	_, err := repo.InsertSnapshot(ctx, repository.InsertSnapshotInput{
		AsOfDate: asOf, KpiName: constants.KpiOSA,
		ScopeType: constants.ScopeTypeGlobal, Value: 0.5,
	})
	require.NoError(t, err)

	deleted, err := repo.DeleteSnapshotsForDate(ctx, asOf, []string{constants.KpiOSA})
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
}
