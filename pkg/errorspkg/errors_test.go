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
