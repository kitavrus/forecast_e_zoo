package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/service"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

type stubRepo struct {
	listErr error
	listOut []models.SendAttempt
}

func (s *stubRepo) ListSendAttempts(_ context.Context, _ models.SendAttemptFilter) ([]models.SendAttempt, string, error) {
	return s.listOut, "", s.listErr
}
func (s *stubRepo) GetSendAttemptByID(_ context.Context, _ uuid.UUID) (models.SendAttempt, error) {
	return models.SendAttempt{}, errorspkg.ErrSendAttemptNotFound
}
func (s *stubRepo) ListChannelConfigs(_ context.Context) ([]models.SupplierChannelConfig, error) {
	return nil, nil
}
func (s *stubRepo) UpsertChannelConfig(_ context.Context, in models.UpsertChannelConfigInput) (models.SupplierChannelConfig, error) {
	return models.SupplierChannelConfig{
		SupplierID:  in.SupplierID,
		ChannelType: in.ChannelType,
	}, nil
}

type stubRouter struct {
	id    uuid.UUID
	stat  string
	ext   *string
	err   error
}

func (s *stubRouter) SendByID(_ context.Context, _ uuid.UUID) (uuid.UUID, string, *string, error) {
	return s.id, s.stat, s.ext, s.err
}

type stubTrigger struct {
	runID   uuid.UUID
	started bool
	err     error
}

func (s *stubTrigger) TryTrigger(_ context.Context, _ int) (uuid.UUID, bool, error) {
	return s.runID, s.started, s.err
}

func TestService_TriggerSendAll_NoSchedulerReturnsUnavailable(t *testing.T) {
	t.Parallel()
	svc := service.New(&stubRepo{}, &stubRouter{}, nil)
	_, _, err := svc.TriggerSendAll(context.Background(), 100)
	require.ErrorIs(t, err, errorspkg.ErrChannelRoutingUnavailable)
}

func TestService_TriggerSendAll_HappyPath(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	svc := service.New(&stubRepo{}, nil, &stubTrigger{runID: id, started: true})
	got, started, err := svc.TriggerSendAll(context.Background(), 100)
	require.NoError(t, err)
	require.True(t, started)
	require.Equal(t, id, got)
}

func TestService_RetryByID_NoRouterUnavailable(t *testing.T) {
	t.Parallel()
	svc := service.New(&stubRepo{}, nil, nil)
	_, _, _, err := svc.RetryByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, errorspkg.ErrChannelRoutingUnavailable)
}

func TestService_RetryByID_PassthroughDomainError(t *testing.T) {
	t.Parallel()
	svc := service.New(&stubRepo{}, &stubRouter{err: errorspkg.ErrPONotReadyToSend}, nil)
	_, _, _, err := svc.RetryByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, errorspkg.ErrPONotReadyToSend)
}

func TestService_GetSendAttempt_NotFound(t *testing.T) {
	t.Parallel()
	svc := service.New(&stubRepo{}, nil, nil)
	_, err := svc.GetSendAttempt(context.Background(), uuid.New())
	require.ErrorIs(t, err, errorspkg.ErrSendAttemptNotFound)
}

func TestService_UpsertConfig_HappyPath(t *testing.T) {
	t.Parallel()
	svc := service.New(&stubRepo{}, nil, nil)
	out, err := svc.UpsertConfig(context.Background(), models.UpsertChannelConfigInput{
		SupplierID:  "sup-1",
		ChannelType: "erp_api",
	})
	require.NoError(t, err)
	require.Equal(t, "sup-1", out.SupplierID)
}
