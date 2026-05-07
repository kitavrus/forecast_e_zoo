package loader_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/loader"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
)

// --- mock SourceReader ---

type mockReader struct {
	products       []loader.ErpProduct
	receiptLines   []loader.ErpReceiptLine
	categories     []loader.ErpCategory
	locations      []loader.ErpLocation
	suppliers      []loader.ErpSupplier
	supplierStocks []loader.ErpSupplierStockSnapshot

	productsErr error
}

func (r *mockReader) ReadProducts(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpProduct], error) {
	if r.productsErr != nil {
		return nil, r.productsErr
	}
	return newSliceIter(r.products), nil
}
func (r *mockReader) ReadProductBarcodes(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpProductBarcode], error) {
	return newSliceIter[loader.ErpProductBarcode](nil), nil
}
func (r *mockReader) ReadCategory(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpCategory], error) {
	return newSliceIter(r.categories), nil
}
func (r *mockReader) ReadLocation(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpLocation], error) {
	return newSliceIter(r.locations), nil
}
func (r *mockReader) ReadSupplier(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpSupplier], error) {
	return newSliceIter(r.suppliers), nil
}
func (r *mockReader) ReadSupplySpec(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpSupplySpec], error) {
	return newSliceIter[loader.ErpSupplySpec](nil), nil
}
func (r *mockReader) ReadPromo(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpPromo], error) {
	return newSliceIter[loader.ErpPromo](nil), nil
}
func (r *mockReader) ReadOrderRule(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpOrderRule], error) {
	return newSliceIter[loader.ErpOrderRule](nil), nil
}
func (r *mockReader) ReadSupplyPlan(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpSupplyPlan], error) {
	return newSliceIter[loader.ErpSupplyPlan](nil), nil
}
func (r *mockReader) ReadStoreAssortment(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpStoreAssortment], error) {
	return newSliceIter[loader.ErpStoreAssortment](nil), nil
}
func (r *mockReader) ReadStoreAssortmentLifecycleEvents(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpStoreAssortmentLifecycleEvent], error) {
	return newSliceIter[loader.ErpStoreAssortmentLifecycleEvent](nil), nil
}
func (r *mockReader) ReadMasterChangeLog(_ context.Context, _ time.Time) (loader.PageIterator[loader.ErpMasterChangeLog], error) {
	return newSliceIter[loader.ErpMasterChangeLog](nil), nil
}
func (r *mockReader) ReadReceiptLine(_ context.Context, _ time.Time, _ time.Time) (loader.PageIterator[loader.ErpReceiptLine], error) {
	return newSliceIter(r.receiptLines), nil
}
func (r *mockReader) ReadLocationStockSnapshot(_ context.Context, _ time.Time, _ time.Time) (loader.PageIterator[loader.ErpLocationStockSnapshot], error) {
	return newSliceIter[loader.ErpLocationStockSnapshot](nil), nil
}
func (r *mockReader) ReadStockMovement(_ context.Context, _ time.Time, _ time.Time) (loader.PageIterator[loader.ErpStockMovement], error) {
	return newSliceIter[loader.ErpStockMovement](nil), nil
}
func (r *mockReader) ReadSupplierStockSnapshot(_ context.Context, _ time.Time, _ time.Time) (loader.PageIterator[loader.ErpSupplierStockSnapshot], error) {
	return newSliceIter(r.supplierStocks), nil
}
func (r *mockReader) Close(_ context.Context) error { return nil }

// sliceIter — generic iterator для тестов. Дублирует logic loader.sliceIterator,
// но тут он публичный для тестов.
type sliceIter[T any] struct {
	items  []T
	idx    int
	cur    T
	closed bool
}

func newSliceIter[T any](items []T) *sliceIter[T] { return &sliceIter[T]{items: items, idx: -1} }
func (s *sliceIter[T]) Next(_ context.Context) bool {
	if s.closed {
		return false
	}
	s.idx++
	if s.idx >= len(s.items) {
		var z T
		s.cur = z
		return false
	}
	s.cur = s.items[s.idx]
	return true
}
func (s *sliceIter[T]) Item() T     { return s.cur }
func (s *sliceIter[T]) Err() error  { return nil }
func (s *sliceIter[T]) Close() error { s.closed = true; return nil }

// --- mock LoaderAPI ---

type mockTx struct {
	committed atomic.Bool
	rolledBack atomic.Bool
}

func (m *mockTx) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }
func (m *mockTx) BeginFunc(_ context.Context, _ func(pgx.Tx) error) error {
	return nil
}
func (m *mockTx) Commit(_ context.Context) error {
	m.committed.Store(true)
	return nil
}
func (m *mockTx) Rollback(_ context.Context) error {
	m.rolledBack.Store(true)
	return nil
}
func (m *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (m *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (m *mockTx) LargeObjects() pgx.LargeObjects                              { return pgx.LargeObjects{} }
func (m *mockTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (m *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (m *mockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }
func (m *mockTx) Conn() *pgx.Conn                                                { return nil }

type mockRepo struct {
	mu sync.Mutex

	insertRunningCalled  int
	upsertCategoryCalled int
	upsertSupplierCalled int
	upsertLocationCalled int
	upsertProductCalled  int
	insertBatchCalled    int
	insertRejectCalled   int
	flipCalled           int
	markCommittedCalled  int
	markFailedCalled     int

	insertReturns models.Load
	flipErr       error
	commitErr     error
	upsertErr     error

	failedReasons []string
}

func (m *mockRepo) BeginTx(_ context.Context) (pgx.Tx, error) { return &mockTx{}, nil }

func (m *mockRepo) InsertRunning(_ context.Context, _ string) (models.Load, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertRunningCalled++
	if m.insertReturns.ID == uuid.Nil {
		m.insertReturns = models.Load{ID: uuid.New(), Status: models.LoadStatusRunning, StartedAt: time.Now()}
	}
	return m.insertReturns, nil
}
func (m *mockRepo) MarkCommitted(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ int64, _ int64, _ []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markCommittedCalled++
	return m.commitErr
}
func (m *mockRepo) MarkFailed(_ context.Context, _ uuid.UUID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markFailedCalled++
	m.failedReasons = append(m.failedReasons, reason)
	return nil
}
func (m *mockRepo) Flip(_ context.Context, _ pgx.Tx, _ uuid.UUID) (models.SnapshotPointer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flipCalled++
	if m.flipErr != nil {
		return models.SnapshotPointer{}, m.flipErr
	}
	return models.SnapshotPointer{}, nil
}
func (m *mockRepo) InsertReject(_ context.Context, _ repository.RejectInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertRejectCalled++
	return nil
}
func (m *mockRepo) UpsertCategory(_ context.Context, _ pgx.Tx, _ repository.CategoryRow, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCategoryCalled++
	return nil
}
func (m *mockRepo) UpsertSupplier(_ context.Context, _ pgx.Tx, _ repository.SupplierRow, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertSupplierCalled++
	return nil
}
func (m *mockRepo) UpsertLocation(_ context.Context, _ pgx.Tx, _ repository.LocationRow, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertLocationCalled++
	return nil
}
func (m *mockRepo) UpsertProduct(_ context.Context, _ pgx.Tx, _ repository.ProductRow, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertProductCalled++
	return m.upsertErr
}
func (m *mockRepo) InsertReceiptLineBatch(_ context.Context, _ pgx.Tx, batch []repository.ReceiptLineRow, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertBatchCalled++
	_ = batch
	return nil
}

// _ used in TestLoader_FlipFailure_MarksFailed to construct typed errors.
var _ = errors.New
