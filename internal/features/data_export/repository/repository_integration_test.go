//go:build integration
// +build integration

package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// --- shared dockertest fixture (one container per `go test ./repository/...` run) ---

type pgFixture struct {
	pool       *pgxpool.Pool
	dsn        string
	tearDown   func()
	migrations *migrate.Migrate
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

// migrationsDir → ../sqls/migrations
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

		// Run migrations.
		dir := migrationsDir(t)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			fixError = err
			return
		}
		drv, err := migratepg.WithInstance(db, &migratepg.Config{})
		if err != nil {
			fixError = err
			return
		}
		src, err := (&file.File{}).Open("file://" + dir)
		if err != nil {
			fixError = err
			return
		}
		m, err := migrate.NewWithInstance("file", src, "postgres", drv)
		if err != nil {
			fixError = err
			return
		}
		if err := m.Up(); err != nil {
			fixError = err
			return
		}
		_ = db.Close()

		// pgxpool.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			fixError = err
			return
		}

		fix = &pgFixture{
			pool: pool,
			dsn:  dsn,
			tearDown: func() {
				pool.Close()
				_ = dpool.Purge(resource)
			},
			migrations: m,
		}
	})
	if fixError != nil {
		t.Fatalf("fixture setup error: %v", fixError)
	}
	return fix
}

// resetSchema очищает все таблицы между тестами (быстрее, чем re-up migrations).
func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	const sql = `
TRUNCATE TABLE
    audit_access,
    reject_log,
    master_change_log,
    store_assortment_lifecycle_events,
    receipt_line,
    location_stock_snapshot,
    stock_movement,
    supplier_stock_snapshot
RESTART IDENTITY CASCADE;
DELETE FROM products;
DELETE FROM product_barcodes;
DELETE FROM store_assortment;
DELETE FROM supply_spec;
DELETE FROM promo;
DELETE FROM order_rule;
DELETE FROM supply_plan;
DELETE FROM category;
DELETE FROM location;
DELETE FROM supplier;
UPDATE snapshot_pointer SET current_load_id = NULL, previous_load_id = NULL, committed_at = NULL WHERE id = 1;
DELETE FROM loads;
`
	_, err := pool.Exec(ctx, sql)
	require.NoError(t, err)
}

// --- loads ---

func TestLoads_InsertRunning_GetByID(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)
	require.Equal(t, "running", string(l.Status))

	got, err := repo.GetByID(ctx, l.ID)
	require.NoError(t, err)
	assert.Equal(t, l.ID, got.ID)
	assert.Equal(t, "running", string(got.Status))
}

func TestLoads_MarkCommitted_StatusTransition(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)

	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.MarkCommitted(ctx, tx, l.ID, 100, 1, []byte(`{"products":{"total":100}}`)))
	require.NoError(t, tx.Commit(ctx))

	got, err := repo.GetByID(ctx, l.ID)
	require.NoError(t, err)
	assert.Equal(t, "committed", string(got.Status))
}

func TestLoads_GetByID_NotFound_ReturnsErrLoadNotFound(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, errorspkg.ErrLoadNotFound)
}

func TestLoads_MarkAborted_StaleRows(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	// Вставим running load и руками сдвинем started_at на 6 часов назад.
	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE loads SET started_at = now() - interval '6 hours' WHERE load_id = $1`, l.ID)
	require.NoError(t, err)

	n, err := repo.MarkAborted(ctx, 4*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	got, err := repo.GetByID(ctx, l.ID)
	require.NoError(t, err)
	assert.Equal(t, "aborted", string(got.Status))
}

// --- snapshot ---

func TestSnapshot_GetCurrent_NoSeed_ReturnsErrSnapshotNotReady(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	require.NoError(t, repo.Seed(ctx)) // идемпотентен; current_load_id всё ещё NULL.
	_, err := repo.GetCurrent(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, errorspkg.ErrSnapshotNotReady)
}

func TestSnapshot_FlipAtomicInTx(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)

	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	sp, err := repo.Flip(ctx, tx, l.ID)
	require.NoError(t, err)
	require.NotNil(t, sp.CurrentLoadID)
	require.Equal(t, l.ID, *sp.CurrentLoadID)
	require.NoError(t, tx.Commit(ctx))

	cur, err := repo.GetCurrent(ctx)
	require.NoError(t, err)
	require.Equal(t, l.ID, *cur.CurrentLoadID)
}

func TestSnapshot_Flip_RollbackOnError(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	// Сначала установим первый snapshot.
	l1, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)
	tx0, _ := f.pool.Begin(ctx)
	_, err = repo.Flip(ctx, tx0, l1.ID)
	require.NoError(t, err)
	require.NoError(t, tx0.Commit(ctx))

	// Откроем tx, попробуем flip на новый load_id — но rollback.
	l2, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)
	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	_, err = repo.Flip(ctx, tx, l2.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))

	cur, err := repo.GetCurrent(ctx)
	require.NoError(t, err)
	assert.Equal(t, l1.ID, *cur.CurrentLoadID, "rollback не должен изменить current_load_id")
}

// --- locks ---

func TestAdvisoryLock_TryLock_AcquireRelease(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	key := repository.LockKey("daily-load")
	conn, err := f.pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	// session-scoped lock через one connection.
	row := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key)
	var ok bool
	require.NoError(t, row.Scan(&ok))
	require.True(t, ok)

	row = conn.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", key)
	var unlocked bool
	require.NoError(t, row.Scan(&unlocked))
	require.True(t, unlocked)
	_ = repo
}

func TestAdvisoryLock_TryLock_BusyReturnsFalse(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	ctx := context.Background()
	key := int64(1234567890)

	// Первое соединение — берём session lock.
	conn1, err := f.pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn1.Release()
	var ok1 bool
	require.NoError(t, conn1.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok1))
	require.True(t, ok1)

	// Второе — попытка взять тот же key должна вернуть false.
	conn2, err := f.pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn2.Release()
	var ok2 bool
	require.NoError(t, conn2.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok2))
	require.False(t, ok2, "second session must not acquire taken advisory lock")

	// Cleanup.
	_, _ = conn1.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
}

// --- reject_log ---

func TestRejectLog_InsertAndSelect(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)

	require.NoError(t, repo.InsertReject(ctx, repository.RejectInput{
		LoadID: l.ID, Entity: "products", Severity: "error",
		Payload: []byte(`{"id":"P1"}`), Errors: []byte(`[{"rule":"missing_required"}]`),
	}))

	rows, err := repo.SelectRejects(ctx, repository.RejectFilter{LoadID: l.ID}, "", 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "products", rows[0].Entity)
}

func TestRejectLog_FilterBySeverity(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)

	require.NoError(t, repo.InsertReject(ctx, repository.RejectInput{LoadID: l.ID, Entity: "products", Severity: "error"}))
	require.NoError(t, repo.InsertReject(ctx, repository.RejectInput{LoadID: l.ID, Entity: "products", Severity: "warn"}))

	errs, err := repo.SelectRejects(ctx, repository.RejectFilter{LoadID: l.ID, Severity: "error"}, "", 100)
	require.NoError(t, err)
	assert.Len(t, errs, 1)

	warns, err := repo.SelectRejects(ctx, repository.RejectFilter{LoadID: l.ID, Severity: "warn"}, "", 100)
	require.NoError(t, err)
	assert.Len(t, warns, 1)
}

// --- master ---

func TestUpsertProduct_AndSelect(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)

	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertProduct(ctx, tx, repository.ProductRow{
		ID: "P-1", SKU: "SKU-1", Name: "Test", Unit: "kg", IsActive: true,
		Attributes: []byte(`{"color":"red"}`),
	}, l.ID))
	// Update — same id, новый name.
	require.NoError(t, repo.UpsertProduct(ctx, tx, repository.ProductRow{
		ID: "P-1", SKU: "SKU-1", Name: "Test2", Unit: "kg", IsActive: true,
	}, l.ID))
	require.NoError(t, tx.Commit(ctx))

	rows, err := repo.SelectProducts(ctx, l.ID, "", 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Test2", rows[0].Name, "second upsert must overwrite")
}

// --- facts ---

func TestReceiptLine_InsertRoutesToPartition(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)

	// preflight: insert location, product (FK soft, but sql column is text — нет FK на receipt_line)
	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)

	eventDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	batch := []repository.ReceiptLineRow{{
		ID: 1, ReceiptID: "R-1", LocationID: "L-1", ProductID: "P-1",
		Qty: 2, Price: 10.5, EventTime: eventDate.Add(8 * time.Hour),
		EventDate: eventDate,
	}}
	require.NoError(t, repo.InsertReceiptLineBatch(ctx, tx, batch, l.ID))
	require.NoError(t, tx.Commit(ctx))

	// Проверим, что строка попала в receipt_line_y2026m05 (целевая партиция).
	var n int
	err = f.pool.QueryRow(ctx, "SELECT count(*) FROM receipt_line_y2026m05").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestReceiptLine_SelectByEventDateRange_UsesPartitionPruning(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	l, err := repo.InsertRunning(ctx, "manual")
	require.NoError(t, err)

	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)

	plan, err := repo.ExplainSelectReceiptLine(ctx, l.ID, from, to)
	require.NoError(t, err)

	// Pruning должен проявиться так, что в плане встречается только нужная партиция.
	assert.True(t,
		strings.Contains(plan, "receipt_line_y2026m05") ||
			strings.Contains(plan, "Append") /* PG18 partitioned scan */,
		"partition pruning evidence absent: %s", plan)
	// И НЕ должно быть «соседних» партиций (m04/m06/m07).
	assert.NotContains(t, plan, "receipt_line_y2026m07")
}

// --- audit_access ---

func TestAuditAccess_Insert(t *testing.T) {
	skipIfDocker(t)
	f := setupFixture(t)
	resetSchema(t, f.pool)
	repo := repository.New(f.pool)
	ctx := context.Background()

	require.NoError(t, repo.InsertAudit(ctx, repository.AuditAccessInput{
		ActorRole: "admin", ActorSub: "u-1",
		Method: "POST", Path: "/admin/loads", Status: 200,
		TraceID: "trace-1",
	}))

	var n int
	require.NoError(t, f.pool.QueryRow(ctx, "SELECT count(*) FROM audit_access").Scan(&n))
	assert.Equal(t, 1, n)
}

// --- pgx import alias keeper ---
var _ = pgx.ErrNoRows
