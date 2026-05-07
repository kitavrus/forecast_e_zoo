package middleware

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// LocalsJWTClaims — ключ для c.Locals() под которым лежат распарсенные claims.
const LocalsJWTClaims = "jwt_claims"

// AlgHS256 / AlgRS256 — поддерживаемые алгоритмы.
const (
	AlgHS256 = "HS256"
	AlgRS256 = "RS256"
)

// JWTConfig — настройки JWT-middleware.
//
// Если Alg = HS256 — используется Secret (HMAC).
// Если Alg = RS256 — грузится PublicKeyPath (PEM RSA public key).
type JWTConfig struct {
	Alg            string
	Secret         string
	PublicKeyPath  string
	publicKey      *rsa.PublicKey // кешируется при первом использовании
	publicKeyError error
}

// Claims — наша оболочка над jwt.RegisteredClaims.
//
// Issuer ("iss") используется как роль (см. role.go).
// Subject ("sub") — caller id (для аудита).
type Claims struct {
	jwt.RegisteredClaims
}

// JWT возвращает Fiber-handler, который проверяет Authorization: Bearer <token>
// и кладёт claims в Locals.
//
// Возможные результаты:
//   - токен ОК → c.Next();
//   - нет header / не "Bearer ..." → ErrAuthMissingToken (401);
//   - подпись/срок/alg невалидны → ErrAuthInvalidToken (401).
func JWT(cfg JWTConfig) fiber.Handler {
	// На старте middleware один раз пытаемся прочитать публичный ключ для RS256,
	// чтобы не делать I/O на каждый запрос.
	if cfg.Alg == AlgRS256 && cfg.PublicKeyPath != "" {
		key, err := loadRSAPublicKey(cfg.PublicKeyPath)
		cfg.publicKey = key
		cfg.publicKeyError = err
	}

	return func(c fiber.Ctx) error {
		auth := c.Get(fiber.HeaderAuthorization)
		if auth == "" {
			return errorspkg.WriteJSON(c, errorspkg.ErrAuthMissingToken)
		}

		// Ожидаем "Bearer <token>".
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return errorspkg.WriteJSON(c, errorspkg.ErrAuthMissingToken)
		}

		raw := parts[1]
		claims := &Claims{}

		_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			return keyFunc(cfg, t)
		})
		if err != nil {
			return errorspkg.WriteJSON(c, errorspkg.ErrAuthInvalidToken.Wrap(err))
		}

		// Дополнительная защита: токен без exp считаем невалидным.
		if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
			return errorspkg.WriteJSON(c, errorspkg.ErrAuthInvalidToken)
		}

		c.Locals(LocalsJWTClaims, claims)
		return c.Next()
	}
}

// keyFunc — выбирает ключ проверки в зависимости от alg в заголовке токена
// и в нашей конфигурации. Не позволяет alg-confusion (HS256 vs RS256).
func keyFunc(cfg JWTConfig, t *jwt.Token) (any, error) {
	switch cfg.Alg {
	case AlgHS256:
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		if cfg.Secret == "" {
			return nil, errors.New("JWT secret not configured")
		}
		return []byte(cfg.Secret), nil

	case AlgRS256:
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		if cfg.publicKeyError != nil {
			return nil, cfg.publicKeyError
		}
		if cfg.publicKey == nil {
			return nil, errors.New("RSA public key not loaded")
		}
		return cfg.publicKey, nil

	default:
		return nil, fmt.Errorf("unsupported alg: %q", cfg.Alg)
	}
}

// loadRSAPublicKey читает PEM и парсит его как RSA public key.
//
// Путь приходит из конфига, не из user input — G304 не применим.
func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- путь из ENV-конфига
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("public key: PEM decode failed")
	}

	// Поддерживаем оба формата: PKIX (BEGIN PUBLIC KEY) и PKCS1 (BEGIN RSA PUBLIC KEY).
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key: PKIX, but not RSA")
		}
		return rsaPub, nil
	}
	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return rsaPub, nil
}

// ClaimsFromCtx достаёт claims из Locals. Возвращает (nil, false), если их нет.
//
// Используется в RequireRole и в обработчиках, которым нужен caller id.
func ClaimsFromCtx(c fiber.Ctx) (*Claims, bool) {
	v := c.Locals(LocalsJWTClaims)
	if v == nil {
		return nil, false
	}
	cl, ok := v.(*Claims)
	return cl, ok
}
