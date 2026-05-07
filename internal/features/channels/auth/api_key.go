// Package auth — APIKey provider implementation.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

// APIKeyProvider — MVP реализация. Читает ключ из env var.
//
// auth_credentials_ref в SupplierChannelConfig — имя env var (например, CHANNEL_AUTH_ERP_API).
// Если ref пустой — fallback на constants.AuthCredentialsEnvDefault.
type APIKeyProvider struct {
	lookup SecretLookup
}

// NewAPIKeyProvider создаёт провайдер с дефолтным env-lookup.
func NewAPIKeyProvider() *APIKeyProvider {
	return &APIKeyProvider{lookup: envLookup}
}

// NewAPIKeyProviderWithLookup — для тестов и для будущего vault-интеграции.
func NewAPIKeyProviderWithLookup(lookup SecretLookup) *APIKeyProvider {
	if lookup == nil {
		lookup = envLookup
	}
	return &APIKeyProvider{lookup: lookup}
}

// Mode возвращает constants.AuthModeAPIKey.
func (p *APIKeyProvider) Mode() string { return constants.AuthModeAPIKey }

// Apply устанавливает заголовок Authorization: Bearer <token>.
//
// Если ref пуст — берётся CHANNEL_AUTH_ERP_API. Если значение пустое — ошибка.
func (p *APIKeyProvider) Apply(
	ctx context.Context, req *http.Request, cfg models.SupplierChannelConfig,
) error {
	if req == nil {
		return errors.New("auth/api_key: nil request")
	}
	ref := constants.AuthCredentialsEnvDefault
	if cfg.AuthCredentialsRef != nil && *cfg.AuthCredentialsRef != "" {
		ref = *cfg.AuthCredentialsRef
	}
	tok, err := p.lookup(ctx, ref)
	if err != nil {
		return fmt.Errorf("auth/api_key: lookup %q: %w", ref, err)
	}
	if tok == "" {
		return fmt.Errorf("auth/api_key: empty secret for ref %q", ref)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

// envLookup — стандартный SecretLookup из ENV.
func envLookup(_ context.Context, ref string) (string, error) {
	v := os.Getenv(ref)
	return v, nil
}

// NotImplementedProvider — заглушка для oauth2/mtls/none.
//
// Возвращает ошибку при Apply — channel-routing service её мапит в errorspkg.ErrInvalidAuthMode
// (или channel_unavailable, если auth_mode валиден но не реализован).
type NotImplementedProvider struct{ ModeName string }

// Mode возвращает заявленный режим.
func (p *NotImplementedProvider) Mode() string { return p.ModeName }

// Apply всегда возвращает ошибку.
func (p *NotImplementedProvider) Apply(
	_ context.Context, _ *http.Request, _ models.SupplierChannelConfig,
) error {
	return fmt.Errorf("auth/%s: not implemented in MVP", p.ModeName)
}
