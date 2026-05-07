// Package auth — pluggable AuthProvider для channel-routing.
//
// MVP: api_key (через ENV var/vault path).
// Future hooks: oauth2 (client_credentials), mtls, none.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

// Provider — интерфейс получения и применения auth-секретов к HTTP-запросу.
//
// Реализации: APIKeyProvider (MVP), OAuth2Provider (заглушка), MTLSProvider (заглушка).
type Provider interface {
	// Apply устанавливает Authorization/credentials на req. Не модифицирует ctx/req.URL.
	Apply(ctx context.Context, req *http.Request, cfg models.SupplierChannelConfig) error

	// Mode возвращает auth_mode из constants (api_key|oauth2|mtls|none).
	Mode() string
}

// SecretLookup — функция получения секрета по ref. ref может быть env var name или vault path.
type SecretLookup func(ctx context.Context, ref string) (string, error)

// Registry — выбирает Provider по auth_mode.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry собирает реестр с переданными провайдерами.
func NewRegistry(providers ...Provider) *Registry {
	m := make(map[string]Provider, len(providers))
	for _, p := range providers {
		if p == nil {
			continue
		}
		m[p.Mode()] = p
	}
	return &Registry{providers: m}
}

// Get возвращает провайдер по auth_mode или ErrAuthModeNotSupported.
func (r *Registry) Get(mode string) (Provider, error) {
	if r == nil {
		return nil, errors.New("auth: registry is nil")
	}
	p, ok := r.providers[mode]
	if !ok {
		return nil, fmt.Errorf("auth: mode %q is not supported", mode)
	}
	return p, nil
}

// Compile-time assertion.
var _ = constants.AuthModeAPIKey
