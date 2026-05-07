package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/channels/auth"
	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

func TestAPIKeyProvider_Apply_FromCustomLookup_SetsBearer(t *testing.T) {
	t.Parallel()
	p := auth.NewAPIKeyProviderWithLookup(func(_ context.Context, ref string) (string, error) {
		require.Equal(t, "MY_SUPPLIER_ENV", ref)
		return "secret-token", nil
	})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://x", nil)
	require.NoError(t, err)
	ref := "MY_SUPPLIER_ENV"
	cfg := models.SupplierChannelConfig{AuthMode: constants.AuthModeAPIKey, AuthCredentialsRef: &ref}
	require.NoError(t, p.Apply(context.Background(), req, cfg))
	require.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"))
}

func TestAPIKeyProvider_Apply_EmptyRefUsesDefault(t *testing.T) {
	t.Parallel()
	p := auth.NewAPIKeyProviderWithLookup(func(_ context.Context, ref string) (string, error) {
		require.Equal(t, constants.AuthCredentialsEnvDefault, ref)
		return "default-token", nil
	})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://x", nil)
	require.NoError(t, p.Apply(context.Background(), req, models.SupplierChannelConfig{}))
	require.Equal(t, "Bearer default-token", req.Header.Get("Authorization"))
}

func TestAPIKeyProvider_Apply_EmptySecret_Errors(t *testing.T) {
	t.Parallel()
	p := auth.NewAPIKeyProviderWithLookup(func(_ context.Context, _ string) (string, error) {
		return "", nil
	})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://x", nil)
	require.Error(t, p.Apply(context.Background(), req, models.SupplierChannelConfig{}))
}

func TestRegistry_PicksByMode(t *testing.T) {
	t.Parallel()
	apiK := auth.NewAPIKeyProvider()
	notImpl := &auth.NotImplementedProvider{ModeName: constants.AuthModeOAuth2}
	reg := auth.NewRegistry(apiK, notImpl)

	got, err := reg.Get(constants.AuthModeAPIKey)
	require.NoError(t, err)
	require.Equal(t, constants.AuthModeAPIKey, got.Mode())

	got2, err := reg.Get(constants.AuthModeOAuth2)
	require.NoError(t, err)
	require.Equal(t, constants.AuthModeOAuth2, got2.Mode())

	_, err = reg.Get(constants.AuthModeMTLS)
	require.Error(t, err)
}

func TestNotImplementedProvider_Apply_Errors(t *testing.T) {
	t.Parallel()
	p := &auth.NotImplementedProvider{ModeName: constants.AuthModeMTLS}
	require.Equal(t, constants.AuthModeMTLS, p.Mode())
	require.Error(t, p.Apply(context.Background(), nil, models.SupplierChannelConfig{}))
}
