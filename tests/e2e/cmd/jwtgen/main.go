// Package main — CLI для генерации HS256 JWT токенов в E2E тесте.
//
// Использование:
//
//	go run ./tests/e2e/cmd/jwtgen -role admin-cli -secret dev-secret-change-in-prod
//
// Claim "iss" (issuer) проверяется middleware.RequireRole — туда кладём роль
// (admin-cli / x-flow-etl / it-read).
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	role := flag.String("role", "admin-cli", "JWT issuer claim (role: admin-cli|x-flow-etl|it-read)")
	secret := flag.String("secret", "dev-secret-change-in-prod", "HS256 signing key")
	sub := flag.String("sub", "e2e-runner", "JWT subject claim")
	ttl := flag.Duration("ttl", time.Hour, "Token TTL (default 1h)")
	flag.Parse()

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    *role,
		Subject:   *sub,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(*ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(*secret))
	if err != nil {
		log.Fatalf("sign token: %v", err)
	}
	fmt.Println(signed)
}
