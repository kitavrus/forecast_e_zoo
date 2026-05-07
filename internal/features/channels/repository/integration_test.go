//go:build integration
// +build integration

package repository_test

import (
	"context"
	"database/sql"
	"errors"
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

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/repository"
	chMig "github.com/Kitavrus/e_zoo/internal/features/channels/sqls/migrations"
	ordersMig "github.com/Kitavrus/e_zoo/internal/features/orders/sqls/migrations"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

type pgFixture struct {
	pool     *pgxpool.Pool
	tearDown func()
}

var (
	fixOnce sync.Once
	fix     *pgFixture
	fixErr  error
)

func skipIfNoDocker(t *testing.T) {
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
			fixErr = err
			return
		}
		if err := dpool.Client.Ping(); err != nil {
			fixErr = err
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
			fixErr = err
			return
		}
		hostPort := resource.GetHostPort("5432/tcp")
		dsn := fmt.Sprintf("postgres://test:test@%s/test?sslmode=disable", hostPort)
		dpool.MaxWait = 90 * time.Second
		if err := dpool.Retry(func() error {
			db, openErr := sql.Open("pgx", dsn)
			if openErr != nil {
				return openErr
			}
			defer func() { _ = db.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			return db.PingContext(ctx)
		}); err != nil {
			fixErr = err
			_ = dpool.Purge(resource)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			fixErr = err
			_ = dpool.Purge(resource)
			return
		}

		// Apply orders + channels migrations (orders must be applied first).
		if err := applyMigrationFile(ctx, pool,
			ordersMig.FS.ReadFile, "4001_orders_schema.up.sql"); err != nil {
			fixErr = err
			_ = dpool.Purge(resource)
			return
		}
		if err := applyMigrationFile(ctx, pool,
			chMig.FS.ReadFile, "5001_channels_schema.up.sql"); err != nil {
			fixErr = err
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
	if fixErr != nil {
		t.Skipf("docker fixture unavailable: %v", fixErr)
	}
	return fix
}

func applyMigrationFile(
	ctx context.Context, pool *pgxpool.Pool,
	read func(name string) ([]byte, error), name string,
) error {
	raw, err := read(name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		return fmt.Errorf("apply %s: %w", name, err)
	}
	return nil
}

// TestRepository_UpsertAndListChannelConfigs covers happy path for config CRUD.
func TestRepository_UpsertAndListChannelConfigs(t *testing.T) {
	skipIfNoDocker(t)
	f := setupFixture(t)
	require.NotNil(t, f)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(),
			`TRUNCATE TABLE channels.supplier_channel_config CASCADE`)
	})

	ctx := context.Background()
	repo := repository.New(f.pool)

	in := models.UpsertChannelConfigInput{
		SupplierID:  "SUP-001",
		ChannelType: constants.ChannelTypeErpAPI,
		EndpointURL: "https://erp.example.com",
		AuthMode:    constants.AuthModeAPIKey,
		TimeoutSec:  30,
		RetryMax:    3,
		IsActive:    true,
	}
	got, err := repo.UpsertChannelConfig(ctx, in)
	require.NoError(t, err)
	require.Equal(t, "SUP-001", got.SupplierID)
	require.Equal(t, constants.ChannelTypeErpAPI, got.ChannelType)

	// Idempotent upsert (update path).
	in.EndpointURL = "https://erp2.example.com"
	got2, err := repo.UpsertChannelConfig(ctx, in)
	require.NoError(t, err)
	require.Equal(t, "https://erp2.example.com", got2.EndpointURL)

	list, err := repo.ListChannelConfigs(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	cfg, err := repo.GetSupplierChannelConfig(ctx, "SUP-001")
	require.NoError(t, err)
	require.True(t, cfg.IsActive)

	_, err = repo.GetSupplierChannelConfig(ctx, "SUP-MISSING")
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrChannelNotConfigured))
}

// TestRepository_SendAttempt_Lifecycle insert→finish→get + idempotency.
func TestRepository_SendAttempt_Lifecycle(t *testing.T) {
	skipIfNoDocker(t)
	f := setupFixture(t)
	require.NotNil(t, f)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(),
			`TRUNCATE TABLE channels.send_attempts CASCADE`)
		_, _ = f.pool.Exec(context.Background(),
			`TRUNCATE TABLE orders.purchase_orders CASCADE`)
	})

	ctx := context.Background()
	repo := repository.New(f.pool)

	var poIDStr string
	err := f.pool.QueryRow(ctx, `
		INSERT INTO orders.purchase_orders
		(po_number, plan_id, supplier_id, location_id, total_qty, currency, status)
		VALUES ('PO-TEST-1', gen_random_uuid(), 'SUP-001', 'LOC-001', 10, 'UAH', 'ready_to_send')
		RETURNING id
	`).Scan(&poIDStr)
	require.NoError(t, err)
	poID, err := uuid.Parse(poIDStr)
	require.NoError(t, err)

	id, startedAt, err := repo.InsertSendAttempt(ctx, nil,
		poID, "SUP-001", constants.ChannelTypeErpAPI,
		constants.SendAttemptStatusPending, 0)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.False(t, startedAt.IsZero())

	httpStatus := 200
	requestBody := `{"po":"x"}`
	respBody := `{"ref":"ERP-9001"}`
	externalRef := "ERP-9001"
	require.NoError(t, repo.FinishSendAttempt(ctx, nil, id, startedAt,
		constants.SendAttemptStatusSuccess, &httpStatus,
		&requestBody, &respBody, nil, 0, &externalRef))

	got, err := repo.GetSendAttemptByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, constants.SendAttemptStatusSuccess, got.Status)
	require.NotNil(t, got.ExternalRef)
	require.Equal(t, "ERP-9001", *got.ExternalRef)

	_, ref, found, err := repo.FindExistingSuccessAttempt(ctx, poID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "ERP-9001", *ref)
}

// TestRepository_MarkPOSentTx covers happy path + invalid status case.
func TestRepository_MarkPOSentTx(t *testing.T) {
	skipIfNoDocker(t)
	f := setupFixture(t)
	require.NotNil(t, f)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(),
			`TRUNCATE TABLE orders.purchase_orders CASCADE`)
	})

	ctx := context.Background()
	repo := repository.New(f.pool)

	var poIDStr string
	err := f.pool.QueryRow(ctx, `
		INSERT INTO orders.purchase_orders
		(po_number, plan_id, supplier_id, location_id, total_qty, currency, status)
		VALUES ('PO-TEST-2', gen_random_uuid(), 'SUP-001', 'LOC-001', 10, 'UAH', 'ready_to_send')
		RETURNING id
	`).Scan(&poIDStr)
	require.NoError(t, err)
	poID, err := uuid.Parse(poIDStr)
	require.NoError(t, err)

	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.MarkPOSentTx(ctx, tx, poID, constants.ChannelTypeErpAPI))
	require.NoError(t, tx.Commit(ctx))

	// Second call must fail (PO already 'sent').
	tx2, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx2.Rollback(ctx) }()
	err = repo.MarkPOSentTx(ctx, tx2, poID, constants.ChannelTypeErpAPI)
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrPONotReadyToSend))
}
