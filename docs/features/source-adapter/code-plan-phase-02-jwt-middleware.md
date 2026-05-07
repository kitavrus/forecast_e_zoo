# Phase 02: JWT middleware + role gating

**Цель:** ввести JWT-аутентификацию (HS256 default, RS256 опционально) и role-based gating ролей `x-flow-etl`, `admin-cli`, `it-read`. Middleware подключается на `/v1/*` и `/admin/*` (но реальное подключение произойдёт в фазе 15 при сборке роутера). Здесь только сами middleware и unit-тесты.

**Commit:** `feat(middleware): JWT HS256/RS256 + role gating (x-flow-etl/admin-cli/it-read)`

---

## Files to CREATE

- `internal/middleware/jwt.go` — `func JWT(cfg JWTConfig) fiber.Handler`. Парсит `Authorization: Bearer <token>` через `github.com/golang-jwt/jwt/v5`. Алгоритм: HS256 если `cfg.Alg == "HS256"`, иначе RS256 (грузит публичный ключ из `cfg.PublicKeyPath`). Claims: `iss` (issuer = роль), `sub` (caller id), `exp`, `iat`. На успех — `c.Locals("jwt_claims", parsedClaims)` и `c.Next()`. На fail — возвращает sentinel `errorspkg.ErrAuthMissingToken` (401, code `auth_invalid_token`) или `errorspkg.ErrAuthInvalidToken` (401, тот же code).
- `internal/middleware/role.go` — `func RequireRole(roles ...string) fiber.Handler`. Берёт claims из `c.Locals("jwt_claims")`, сверяет `iss` со списком разрешённых ролей. На fail — `errorspkg.ErrAuthForbidden` (403, code `auth_forbidden`). Хелпер: `RequireXFlowETL()`, `RequireAdmin()`, `RequireITRead()` — чтобы в роутере не дублировать литералы.
- `internal/middleware/request_id.go` — генерация `X-Request-Id` (uuid v7 если есть, иначе uuid v4) + кладём в `c.Locals("trace_id")`. Используется для `traceId` в ErrorResponseJSON.
- `internal/middleware/jwt_test.go` — unit:
  - `TestJWT_NoHeader_Returns401`
  - `TestJWT_MalformedToken_Returns401`
  - `TestJWT_ExpiredToken_Returns401`
  - `TestJWT_HS256_ValidToken_Passes`
  - `TestJWT_RS256_ValidToken_Passes` (генератор ключа в тесте)
  - `TestJWT_ClaimsInLocals` — проверяет, что claims доступны в `c.Locals`.
- `internal/middleware/role_test.go` — unit:
  - `TestRequireRole_MissingClaims_Forbidden`
  - `TestRequireRole_WrongIssuer_Forbidden`
  - `TestRequireRole_AllowedRole_Passes`
  - `TestRequireXFlowETLAllowsXFlow`
  - `TestRequireAdminBlocksXFlow`
- `test/helpers/jwt.go` — `func SignTestJWT(t *testing.T, secret, role, sub string, ttl time.Duration) string` (HS256) и `func SignTestJWTRSA(t *testing.T, key *rsa.PrivateKey, role, sub string, ttl) string`.

## Files to MODIFY

- `pkg/errorspkg/errors.go` — добавить sentinel:
  - `ErrAuthMissingToken` (401, code `auth_invalid_token`, message `"auth required"`)
  - `ErrAuthInvalidToken` (401, code `auth_invalid_token`, message `"invalid token"`)
  - `ErrAuthForbidden` (403, code `auth_forbidden`, message `"forbidden"`)
- `pkg/errorspkg/errors_test.go` — добавить кейсы для трёх новых sentinel.
- `go.mod` / `go.sum` — добавить `github.com/golang-jwt/jwt/v5`, `github.com/google/uuid`.

## SQL/Migrations

— нет.

## Run after

```bash
go mod tidy
make test-unit
make lint
make build
```

## Tests in this phase

См. список выше — итого 11 unit-тестов (6 для JWT + 5 для role + 3 новых для errorspkg).

## Definition of Done

- [ ] `internal/middleware/jwt.go` парсит HS256 и RS256, кладёт claims в Locals.
- [ ] `internal/middleware/role.go` блокирует чужой issuer, пропускает свой.
- [ ] `internal/middleware/request_id.go` генерирует/проксирует `X-Request-Id`.
- [ ] Все sentinel-ошибки `ErrAuthMissingToken`, `ErrAuthInvalidToken`, `ErrAuthForbidden` определены.
- [ ] `make test-unit` зелёный (новые тесты + старые из фазы 01).
- [ ] `make lint` без ошибок.
- [ ] `make build` без ошибок.
- [ ] Коммит атомарный, сообщение `feat(middleware): ...`.
