package errorspkg

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrBadRequest_Defaults — проверяем дефолтные поля sentinel'а.
func TestErrBadRequest_Defaults(t *testing.T) {
	t.Parallel()

	require.NotNil(t, ErrBadRequest)
	assert.Equal(t, "bad_request", ErrBadRequest.Code)
	assert.Equal(t, http.StatusBadRequest, ErrBadRequest.HTTP)
	assert.Equal(t, SupportBadRequest, ErrBadRequest.SupportMessage)
}

// TestErrNotFound_WrappedIs — проверяем, что errors.Is корректно сравнивает по Code,
// даже когда экземпляр обёрнут через .Wrap(cause).
func TestErrNotFound_WrappedIs(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("row missing in db")
	wrapped := ErrNotFound.Wrap(cause)

	assert.True(t, errors.Is(wrapped, ErrNotFound), "errors.Is должен находить sentinel по Code")
	assert.False(t, errors.Is(wrapped, ErrBadRequest), "разные sentinel-ы не должны совпадать")
}

// TestAuthSentinels_HTTP — проверяем коды и HTTP-статусы JWT-sentinel'ов.
func TestAuthSentinels_HTTP(t *testing.T) {
	t.Parallel()

	assert.Equal(t, http.StatusUnauthorized, ErrAuthMissingToken.HTTP)
	assert.Equal(t, http.StatusUnauthorized, ErrAuthInvalidToken.HTTP)
	assert.Equal(t, http.StatusForbidden, ErrAuthForbidden.HTTP)

	// missing и invalid должны иметь одинаковый Code, чтобы клиент не различал кейсы.
	assert.Equal(t, ErrAuthMissingToken.Code, ErrAuthInvalidToken.Code)
	assert.Equal(t, "auth_invalid_token", ErrAuthMissingToken.Code)
	assert.Equal(t, "auth_forbidden", ErrAuthForbidden.Code)

	// errors.Is должен правильно различать sentinel'ы.
	wrappedMissing := ErrAuthMissingToken.Wrap(fmt.Errorf("no header"))
	assert.True(t, errors.Is(wrappedMissing, ErrAuthMissingToken))
	// А вот разные sentinel'ы с одинаковым Code считаем равными по Is — это ОК,
	// потому что доменно это одна категория ошибки. Проверяем, что не путается с Forbidden.
	assert.False(t, errors.Is(wrappedMissing, ErrAuthForbidden))
}

// TestError_WithDetails_Idempotent — проверяем, что WithDetails не мутирует оригинал.
func TestError_WithDetails_Idempotent(t *testing.T) {
	t.Parallel()

	original := ErrBadRequest
	withFields := original.WithDetails(Detail{Field: "x", Rule: "required"})

	require.NotNil(t, withFields)
	assert.Empty(t, original.Details, "оригинал sentinel не должен мутироваться")
	assert.Len(t, withFields.Details, 1)
	assert.Equal(t, "x", withFields.Details[0].Field)
}
