package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/repository"
	"github.com/Kitavrus/e_zoo/internal/features/orders/service"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// stubRepo реализует service.Repo без БД (нужен только для проверки
// happy/fast-path логики, не build-pipeline; build тестируется через integration).
type stubRepo struct {
	getResult    models.PurchaseOrder
	getErr       error
	cancelResult models.PurchaseOrder
	cancelErr    error
}

func (s *stubRepo) SelectApprovedPlansForUpdate(_ context.Context, _ pgx.Tx, _ int) ([]models.ApprovedPlan, error) {
	return nil, errors.New("stub: not used")
}
func (s *stubRepo) SelectPlanLinesForBuild(_ context.Context, _ uuid.UUID, _, _ string) ([]models.PlanLine, error) {
	return nil, errors.New("stub: not used")
}
func (s *stubRepo) GetSupplierMaster(_ context.Context, id string) (models.SupplierMaster, error) {
	return models.SupplierMaster{SupplierID: id}, nil
}
func (s *stubRepo) GetProductMaster(_ context.Context, id string) (models.ProductMaster, error) {
	return models.ProductMaster{ProductID: id}, nil
}
func (s *stubRepo) NextSequenceTx(_ context.Context, _ pgx.Tx) (int64, error) { return 1, nil }
func (s *stubRepo) InsertPurchaseOrderTx(_ context.Context, _ pgx.Tx, in repository.InsertPOInput) (models.PurchaseOrder, error) {
	return models.PurchaseOrder{ID: uuid.New(), PONumber: in.PONumber, Status: constants.POStatusReadyToSend}, nil
}
func (s *stubRepo) InsertPOLinesBulkTx(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ []repository.BulkLine) error {
	return nil
}
func (s *stubRepo) InsertPOStatusHistoryTx(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ *string, _ string, _ *string, _ *string) error {
	return nil
}
func (s *stubRepo) MarkPlanConvertedTx(_ context.Context, _ pgx.Tx, _ uuid.UUID) error {
	return nil
}
func (s *stubRepo) GetPOByID(_ context.Context, _ uuid.UUID) (models.PurchaseOrder, error) {
	return s.getResult, s.getErr
}
func (s *stubRepo) GetPOLines(_ context.Context, _ uuid.UUID) ([]models.POLine, error) {
	return nil, nil
}
func (s *stubRepo) GetPOHistory(_ context.Context, _ uuid.UUID) ([]models.POStatusHistory, error) {
	return nil, nil
}
func (s *stubRepo) ListPOs(_ context.Context, _ models.POFilter) ([]models.PurchaseOrder, string, error) {
	return nil, "", nil
}
func (s *stubRepo) CancelPO(_ context.Context, _ uuid.UUID, _ string) (models.PurchaseOrder, error) {
	return s.cancelResult, s.cancelErr
}
func (s *stubRepo) CancelPOTx(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ string) (models.PurchaseOrder, error) {
	return s.cancelResult, s.cancelErr
}

func TestService_TriggerBuild_NoTriggerReturnsUnavailable(t *testing.T) {
	t.Parallel()
	svc := service.New(&stubRepo{}, nil, nil, nil)
	_, _, err := svc.TriggerBuild(context.Background(), 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, errorspkg.ErrOrderBuilderUnavailable)
}

func TestService_GetPOWithDetails_NotFound(t *testing.T) {
	t.Parallel()
	svc := service.New(&stubRepo{getErr: errorspkg.ErrPurchaseOrderNotFound}, nil, nil, nil)
	_, err := svc.GetPOWithDetails(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, errorspkg.ErrPurchaseOrderNotFound)
}

func TestService_Cancel_NoPool_FallbackToCancelPO(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	expected := models.PurchaseOrder{ID: id, Status: constants.POStatusCancelled}
	svc := service.New(&stubRepo{cancelResult: expected}, nil, nil, nil)
	got, err := svc.Cancel(context.Background(), models.CancelInput{POID: id, Reason: "test"})
	require.NoError(t, err)
	assert.Equal(t, expected.Status, got.Status)
}
