package router_svc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/router_svc"
	"github.com/Kitavrus/e_zoo/internal/features/channels/sender"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// fakeRepo реализует router_svc.Repo, но НЕ выполняет реальные SQL —
// только эмулирует выходные данные. Tx-операции игнорируем (tx может быть nil).
type fakeRepo struct {
	pos               []models.PurchaseOrderForSend
	cfg               models.SupplierChannelConfig
	cfgErr            error
	getPOErr          error
	getPOStatus       string
	insertCalls       int
	finishCalls       int
	markSentCalls     int
	findExistingHit   bool
	findExistingExtRef *string
	failMark          bool
}

func (f *fakeRepo) SelectReadyToSendPOsTx(_ context.Context, _ pgx.Tx, _ int) ([]models.PurchaseOrderForSend, error) {
	return f.pos, nil
}

func (f *fakeRepo) GetPOByIDForSend(_ context.Context, id uuid.UUID) (models.PurchaseOrderForSend, string, error) {
	if f.getPOErr != nil {
		return models.PurchaseOrderForSend{}, "", f.getPOErr
	}
	for _, p := range f.pos {
		if p.ID == id {
			return p, f.getPOStatus, nil
		}
	}
	return models.PurchaseOrderForSend{}, "", errorspkg.ErrPurchaseOrderNotFound
}

func (f *fakeRepo) GetSupplierChannelConfig(_ context.Context, _ string) (models.SupplierChannelConfig, error) {
	if f.cfgErr != nil {
		return models.SupplierChannelConfig{}, f.cfgErr
	}
	return f.cfg, nil
}

func (f *fakeRepo) InsertSendAttempt(_ context.Context, _ pgx.Tx,
	_ uuid.UUID, _, _, _ string, _ int,
) (uuid.UUID, time.Time, error) {
	f.insertCalls++
	return uuid.New(), time.Now().UTC(), nil
}

func (f *fakeRepo) FinishSendAttempt(_ context.Context, _ pgx.Tx,
	_ uuid.UUID, _ time.Time, _ string, _ *int,
	_, _, _ *string, _ int, _ *string,
) error {
	f.finishCalls++
	return nil
}

func (f *fakeRepo) MarkPOSentTx(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ string) error {
	f.markSentCalls++
	if f.failMark {
		return errorspkg.ErrPONotReadyToSend
	}
	return nil
}

func (f *fakeRepo) FindExistingSuccessAttempt(_ context.Context, _ uuid.UUID) (uuid.UUID, *string, bool, error) {
	if f.findExistingHit {
		return uuid.New(), f.findExistingExtRef, true, nil
	}
	return uuid.Nil, nil, false, nil
}

// fakeSender — настраиваемый ChannelSender для тестов router-а.
type fakeSender struct {
	channel string
	result  sender.SendResult
	err     error
}

func (s *fakeSender) ChannelType() string { return s.channel }
func (s *fakeSender) Send(_ context.Context, _ sender.SendInput, _ models.SupplierChannelConfig) (sender.SendResult, error) {
	return s.result, s.err
}

// fakeRegistry — regression-aware registry без зависимости от реального sender.NewRegistry.
type fakeRegistry struct {
	senders map[string]sender.ChannelSender
}

func (r *fakeRegistry) Get(ch string) (sender.ChannelSender, error) {
	s, ok := r.senders[ch]
	if !ok {
		return nil, errorspkg.ErrInvalidChannelType
	}
	return s, nil
}

// SendByID не использует pool/tx (только GetPOByIDForSend + idempotency lookup).
// Поэтому проверим idempotent path — никаких pool calls, всё через repo.

func TestChannelRouter_SendByID_IdempotentReturnsExisting(t *testing.T) {
	t.Parallel()
	po := models.PurchaseOrderForSend{
		ID: uuid.New(), PONumber: "PO-1", SupplierID: "s1", LocationID: "l1",
		Currency: "UAH", CreatedAt: time.Now().UTC(),
	}
	ext := "ERP-EXISTS"
	repo := &fakeRepo{
		pos: []models.PurchaseOrderForSend{po},
		cfg: models.SupplierChannelConfig{
			SupplierID: "s1", ChannelType: constants.ChannelTypeErpAPI, IsActive: true,
		},
		getPOStatus:        constants.POStatusSent,
		findExistingHit:    true,
		findExistingExtRef: &ext,
	}
	reg := &fakeRegistry{senders: map[string]sender.ChannelSender{
		constants.ChannelTypeErpAPI: &fakeSender{channel: constants.ChannelTypeErpAPI},
	}}
	rt := router_svc.New(repo, nil, reg, nil, router_svc.Metrics{})
	id, st, retExt, err := rt.SendByID(context.Background(), po.ID)
	require.NoError(t, err)
	require.Equal(t, constants.SendAttemptStatusSuccess, st)
	require.NotEqual(t, uuid.Nil, id)
	require.NotNil(t, retExt)
	require.Equal(t, "ERP-EXISTS", *retExt)
}

func TestChannelRouter_SendByID_PONotReadyToSend(t *testing.T) {
	t.Parallel()
	po := models.PurchaseOrderForSend{ID: uuid.New(), SupplierID: "s1"}
	repo := &fakeRepo{
		pos:         []models.PurchaseOrderForSend{po},
		cfg:         models.SupplierChannelConfig{SupplierID: "s1", ChannelType: constants.ChannelTypeErpAPI},
		getPOStatus: constants.POStatusCancelled,
	}
	reg := &fakeRegistry{senders: map[string]sender.ChannelSender{
		constants.ChannelTypeErpAPI: &fakeSender{channel: constants.ChannelTypeErpAPI},
	}}
	rt := router_svc.New(repo, nil, reg, nil, router_svc.Metrics{})
	_, _, _, err := rt.SendByID(context.Background(), po.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, errorspkg.ErrPONotReadyToSend)
}

func TestChannelRouter_SendByID_NotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{getPOErr: errorspkg.ErrPurchaseOrderNotFound}
	reg := &fakeRegistry{senders: map[string]sender.ChannelSender{}}
	rt := router_svc.New(repo, nil, reg, nil, router_svc.Metrics{})
	_, _, _, err := rt.SendByID(context.Background(), uuid.New())
	require.Error(t, err)
	require.ErrorIs(t, err, errorspkg.ErrPurchaseOrderNotFound)
}
