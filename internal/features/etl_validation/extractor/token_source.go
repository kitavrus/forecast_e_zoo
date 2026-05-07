// Package extractor реализует HTTP-клиент к source-adapter (Модуль 1):
// JWT bearer auth, retry с экспоненциальным backoff, ETag/If-None-Match,
// NDJSON streaming.
package extractor

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenSource — источник JWT-токена для Authorization: Bearer.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// HS256TokenSource выпускает HS256-подписанные токены.
//
// Кэширует токен до cachedExp - cacheLeeway.
type HS256TokenSource struct {
	signingKey []byte
	role       string
	issuer     string
	ttl        time.Duration
	leeway     time.Duration
	now        func() time.Time

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

// HS256Config — параметры HS256TokenSource.
type HS256Config struct {
	SigningKey []byte
	Role       string
	Issuer     string
	TTL        time.Duration // время жизни токена; default 1h
	Leeway     time.Duration // зазор «обновить токен раньше exp»; default 5m
}

// NewHS256TokenSource создаёт TokenSource c HMAC-SHA256.
func NewHS256TokenSource(cfg HS256Config) (*HS256TokenSource, error) {
	if len(cfg.SigningKey) == 0 {
		return nil, errors.New("extractor: HS256 signing key is empty")
	}
	if cfg.Role == "" {
		cfg.Role = "x-flow-etl"
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "etl"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = time.Hour
	}
	if cfg.Leeway <= 0 {
		cfg.Leeway = 5 * time.Minute
	}
	return &HS256TokenSource{
		signingKey: cfg.SigningKey,
		role:       cfg.Role,
		issuer:     cfg.Issuer,
		ttl:        cfg.TTL,
		leeway:     cfg.Leeway,
		now:        time.Now,
	}, nil
}

// Token возвращает кэшированный либо свежий JWT.
func (s *HS256TokenSource) Token(_ context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.cached != "" && now.Before(s.cachedExp.Add(-s.leeway)) {
		return s.cached, nil
	}
	exp := now.Add(s.ttl)
	claims := jwt.MapClaims{
		"role": s.role,
		"iss":  s.issuer,
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.signingKey)
	if err != nil {
		return "", fmt.Errorf("extractor: HS256 sign: %w", err)
	}
	s.cached = signed
	s.cachedExp = exp
	return signed, nil
}

// RS256TokenSource выпускает RS256-подписанные токены через RSA private key.
type RS256TokenSource struct {
	pk     *rsa.PrivateKey
	role   string
	issuer string
	ttl    time.Duration
	leeway time.Duration
	now    func() time.Time

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

// RS256Config — параметры RS256TokenSource.
type RS256Config struct {
	PrivateKey *rsa.PrivateKey
	Role       string
	Issuer     string
	TTL        time.Duration
	Leeway     time.Duration
}

// NewRS256TokenSource создаёт RS256 TokenSource.
func NewRS256TokenSource(cfg RS256Config) (*RS256TokenSource, error) {
	if cfg.PrivateKey == nil {
		return nil, errors.New("extractor: RS256 private key is nil")
	}
	if cfg.Role == "" {
		cfg.Role = "x-flow-etl"
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "etl"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = time.Hour
	}
	if cfg.Leeway <= 0 {
		cfg.Leeway = 5 * time.Minute
	}
	return &RS256TokenSource{
		pk:     cfg.PrivateKey,
		role:   cfg.Role,
		issuer: cfg.Issuer,
		ttl:    cfg.TTL,
		leeway: cfg.Leeway,
		now:    time.Now,
	}, nil
}

// Token возвращает кэшированный либо свежий RS256 JWT.
func (s *RS256TokenSource) Token(_ context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.cached != "" && now.Before(s.cachedExp.Add(-s.leeway)) {
		return s.cached, nil
	}
	exp := now.Add(s.ttl)
	claims := jwt.MapClaims{
		"role": s.role,
		"iss":  s.issuer,
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(s.pk)
	if err != nil {
		return "", fmt.Errorf("extractor: RS256 sign: %w", err)
	}
	s.cached = signed
	s.cachedExp = exp
	return signed, nil
}

// StaticTokenSource — для тестов: возвращает фиксированную строку.
type StaticTokenSource struct{ Value string }

// Token реализует интерфейс TokenSource.
func (s StaticTokenSource) Token(_ context.Context) (string, error) { return s.Value, nil }
