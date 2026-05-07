package dto

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// extractEnums извлекает значения тега `enums:"a,b,c"` для поля fieldName из t.
func extractEnums(t reflect.Type, fieldName string) []string {
	f, ok := t.FieldByName(fieldName)
	if !ok {
		return nil
	}
	tag := f.Tag.Get("enums")
	if tag == "" {
		return nil
	}
	out := strings.Split(tag, ",")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

func sortedCopy(in []string) []string {
	cp := append([]string(nil), in...)
	sort.Strings(cp)
	return cp
}

// TestEtlRunResponse_StatusEnumSync — sync-тест: enums:"..."
// у поля Status совпадает с constants.EtlRunStatuses.
func TestEtlRunResponse_StatusEnumSync(t *testing.T) {
	t.Parallel()
	got := extractEnums(reflect.TypeOf(EtlRunResponse{}), "Status")
	assert.Equal(t, sortedCopy(constants.EtlRunStatuses), sortedCopy(got),
		"EtlRunResponse.Status enums:\"...\" must match constants.EtlRunStatuses")
}

// TestEtlRunResponse_KindEnumSync — sync-тест поля Kind.
func TestEtlRunResponse_KindEnumSync(t *testing.T) {
	t.Parallel()
	got := extractEnums(reflect.TypeOf(EtlRunResponse{}), "Kind")
	assert.Equal(t, sortedCopy(constants.EtlRunKinds), sortedCopy(got))
}

// TestEtlRunResponse_TriggerEnumSync — sync-тест поля Trigger.
func TestEtlRunResponse_TriggerEnumSync(t *testing.T) {
	t.Parallel()
	got := extractEnums(reflect.TypeOf(EtlRunResponse{}), "Trigger")
	assert.Equal(t, sortedCopy(constants.EtlRunTriggers), sortedCopy(got))
}

// TestMartRefreshResponse_StatusEnumSync — sync-тест поля Status.
func TestMartRefreshResponse_StatusEnumSync(t *testing.T) {
	t.Parallel()
	got := extractEnums(reflect.TypeOf(MartRefreshResponse{}), "Status")
	assert.Equal(t, sortedCopy(constants.EtlRunStatuses), sortedCopy(got))
}
