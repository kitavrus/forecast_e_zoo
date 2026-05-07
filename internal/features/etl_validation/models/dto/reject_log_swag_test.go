package dto

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// TestRejectLogEntryResponse_SeverityEnumSync — sync-тест поля Severity.
func TestRejectLogEntryResponse_SeverityEnumSync(t *testing.T) {
	t.Parallel()
	got := extractEnums(reflect.TypeOf(RejectLogEntryResponse{}), "Severity")
	assert.Equal(t, sortedCopy(constants.RejectSeverities), sortedCopy(got),
		"RejectLogEntryResponse.Severity enums:\"...\" must match constants.RejectSeverities")
}
