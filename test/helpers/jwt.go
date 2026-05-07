// Package helpers — общие утилиты для тестов уровня модуля.
package helpers

import (
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// SignTestJWT подписывает HS256-токен с указанной ролью (issuer) и subject.
//
// ttl > 0 → exp = now + ttl. ttl <= 0 → токен сразу просрочен (для тестов 401).
func SignTestJWT(t *testing.T, secret, role, sub string, ttl time.Duration) string {
	t.Helper()

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    role,
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// SignTestJWTRSA подписывает RS256-токен.
func SignTestJWTRSA(t *testing.T, key *rsa.PrivateKey, role, sub string, ttl time.Duration) string {
	t.Helper()

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    role,
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}
