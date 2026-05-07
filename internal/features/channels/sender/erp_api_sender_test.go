package sender_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/channels/auth"
	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/formatter"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
	"github.com/Kitavrus/e_zoo/internal/features/channels/sender"
)

type fakeHTTP struct {
	calls    int
	statuses []int
	bodies   []string
	doErr    error
}

func (f *fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	if f.doErr != nil {
		return nil, f.doErr
	}
	idx := f.calls - 1
	if idx >= len(f.statuses) {
		idx = len(f.statuses) - 1
	}
	body := ""
	if idx < len(f.bodies) {
		body = f.bodies[idx]
	}
	return &http.Response{
		StatusCode: f.statuses[idx],
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func newFakeSender(t *testing.T, fc *fakeHTTP) *sender.ErpAPISender {
	t.Helper()
	authP := auth.NewAPIKeyProviderWithLookup(func(_ context.Context, _ string) (string, error) {
		return "test-token", nil
	})
	s, err := sender.NewErpAPISender(sender.Config{
		HTTPClient:   fc,
		AuthProvider: authP,
		Formatter:    formatter.NewJSONFormatter(),
	})
	require.NoError(t, err)
	return s
}

func newCfg() models.SupplierChannelConfig {
	return models.SupplierChannelConfig{
		SupplierID:  "sup-1",
		ChannelType: constants.ChannelTypeErpAPI,
		EndpointURL: "http://example/api/po",
		AuthMode:    constants.AuthModeAPIKey,
		TimeoutSec:  30,
		RetryMax:    2,
		IsActive:    true,
	}
}

func newPO() models.PurchaseOrderForSend {
	return models.PurchaseOrderForSend{
		ID:         uuid.New(),
		PONumber:   "PO-2026-001",
		SupplierID: "sup-1",
		LocationID: "loc-1",
		TotalQty:   12.5,
		Currency:   "UAH",
		CreatedAt:  time.Now().UTC(),
	}
}

func TestErpAPISender_Send_Success_2xx(t *testing.T) {
	t.Parallel()
	fc := &fakeHTTP{statuses: []int{200}, bodies: []string{`{"external_ref":"ERP-42"}`}}
	s := newFakeSender(t, fc)
	res, err := s.Send(context.Background(), sender.SendInput{PO: newPO()}, newCfg())
	require.NoError(t, err)
	require.Equal(t, constants.SendAttemptStatusSuccess, res.Status)
	require.NotNil(t, res.ExternalRef)
	require.Equal(t, "ERP-42", *res.ExternalRef)
	require.NotNil(t, res.HTTPStatusCode)
	require.Equal(t, 200, *res.HTTPStatusCode)
	require.Equal(t, 1, fc.calls)
}

func TestErpAPISender_Send_5xxThen2xx_Retries(t *testing.T) {
	t.Parallel()
	fc := &fakeHTTP{statuses: []int{500, 503, 200}, bodies: []string{"err1", "err2", `{"id":"X-1"}`}}
	s := newFakeSender(t, fc)
	cfg := newCfg()
	cfg.RetryMax = 3
	res, err := s.Send(context.Background(), sender.SendInput{PO: newPO()}, cfg)
	require.NoError(t, err)
	require.Equal(t, constants.SendAttemptStatusSuccess, res.Status)
	require.Equal(t, 2, res.RetryCount)
	require.Equal(t, 3, fc.calls)
}

func TestErpAPISender_Send_4xx_NoRetry(t *testing.T) {
	t.Parallel()
	fc := &fakeHTTP{statuses: []int{400}, bodies: []string{"bad request"}}
	s := newFakeSender(t, fc)
	res, err := s.Send(context.Background(), sender.SendInput{PO: newPO()}, newCfg())
	require.Error(t, err)
	require.Equal(t, constants.SendAttemptStatusFailed, res.Status)
	require.Equal(t, 1, fc.calls)
}

func TestErpAPISender_Send_429_DoesRetry(t *testing.T) {
	t.Parallel()
	fc := &fakeHTTP{statuses: []int{429, 200}, bodies: []string{"slow", `{"id":"OK"}`}}
	s := newFakeSender(t, fc)
	cfg := newCfg()
	cfg.RetryMax = 2
	res, err := s.Send(context.Background(), sender.SendInput{PO: newPO()}, cfg)
	require.NoError(t, err)
	require.Equal(t, constants.SendAttemptStatusSuccess, res.Status)
	require.Equal(t, 2, fc.calls)
}

func TestErpAPISender_Send_NetworkErr_Retries(t *testing.T) {
	t.Parallel()
	fc := &fakeHTTP{doErr: errors.New("connection refused")}
	s := newFakeSender(t, fc)
	cfg := newCfg()
	cfg.RetryMax = 2
	res, err := s.Send(context.Background(), sender.SendInput{PO: newPO()}, cfg)
	require.Error(t, err)
	require.Equal(t, constants.SendAttemptStatusFailed, res.Status)
	require.Equal(t, 3, fc.calls) // initial + 2 retries
}

func TestErpAPISender_Send_EmptyEndpoint_Errors(t *testing.T) {
	t.Parallel()
	fc := &fakeHTTP{statuses: []int{200}}
	s := newFakeSender(t, fc)
	cfg := newCfg()
	cfg.EndpointURL = ""
	_, err := s.Send(context.Background(), sender.SendInput{PO: newPO()}, cfg)
	require.Error(t, err)
}

func TestErpAPISender_ChannelType(t *testing.T) {
	t.Parallel()
	s, err := sender.NewErpAPISender(sender.Config{
		AuthProvider: auth.NewAPIKeyProvider(),
		Formatter:    formatter.NewJSONFormatter(),
	})
	require.NoError(t, err)
	require.Equal(t, constants.ChannelTypeErpAPI, s.ChannelType())
}

func TestRegistry_GetByChannel(t *testing.T) {
	t.Parallel()
	s, err := sender.NewErpAPISender(sender.Config{
		AuthProvider: auth.NewAPIKeyProvider(),
		Formatter:    formatter.NewJSONFormatter(),
	})
	require.NoError(t, err)
	reg := sender.NewRegistry(s)
	got, err := reg.Get(constants.ChannelTypeErpAPI)
	require.NoError(t, err)
	require.NotNil(t, got)

	_, err = reg.Get(constants.ChannelTypeEdiX12)
	require.Error(t, err)
}

func TestNotImplementedSender_ReturnsError(t *testing.T) {
	t.Parallel()
	s := &sender.NotImplementedSender{Channel: constants.ChannelTypeEdiX12}
	require.Equal(t, constants.ChannelTypeEdiX12, s.ChannelType())
	_, err := s.Send(context.Background(), sender.SendInput{}, models.SupplierChannelConfig{})
	require.Error(t, err)
}
