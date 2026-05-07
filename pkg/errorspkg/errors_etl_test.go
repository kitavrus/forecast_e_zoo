package errorspkg

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEtlSentinels_Defaults — проверяем дефолты EV-* sentinel'ов.
func TestEtlSentinels_Defaults(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  *Error
		code string
		http int
		sup  string
	}{
		{"EtlRunAlreadyRunning", ErrEtlRunAlreadyRunning, "etl_run_already_running", http.StatusConflict, "EV-001"},
		{"EtlRunNotFound", ErrEtlRunNotFound, "etl_run_not_found", http.StatusNotFound, "EV-002"},
		{"CannotRetryEtl", ErrCannotRetryEtl, "cannot_retry_etl", http.StatusConflict, "EV-003"},
		{"SourceUnavailable", ErrSourceUnavailable, "source_unavailable", http.StatusBadGateway, "EV-004"},
		{"MartRefreshNotSupported", ErrMartRefreshNotSupported, "mart_refresh_not_supported", http.StatusBadRequest, "EV-005"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NotNil(t, tc.err)
			assert.Equal(t, tc.code, tc.err.Code)
			assert.Equal(t, tc.http, tc.err.HTTP)
			assert.Equal(t, tc.sup, tc.err.SupportMessage)
			wrapped := tc.err.Wrap(fmt.Errorf("cause"))
			assert.True(t, errors.Is(wrapped, tc.err))
		})
	}
}

// TestSupportMessageCodes_ContainsEV — sync-тест: список SupportMessageCodes
// содержит EV-001..EV-005.
func TestSupportMessageCodes_ContainsEV(t *testing.T) {
	t.Parallel()
	required := []string{"EV-001", "EV-002", "EV-003", "EV-004", "EV-005"}
	set := make(map[string]struct{}, len(SupportMessageCodes))
	for _, c := range SupportMessageCodes {
		set[c] = struct{}{}
	}
	for _, code := range required {
		_, ok := set[code]
		assert.True(t, ok, "SupportMessageCodes must contain %s", code)
	}
}

// TestSupportMessageCodes_NoDuplicates — sync-тест: дубликатов нет.
func TestSupportMessageCodes_NoDuplicates(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, len(SupportMessageCodes))
	for _, c := range SupportMessageCodes {
		_, dup := seen[c]
		assert.False(t, dup, "duplicate support code: %s", c)
		seen[c] = struct{}{}
	}
}
