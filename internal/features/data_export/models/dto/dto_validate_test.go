package dto_test

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
)

func newValidator() *validator.Validate {
	return validator.New(validator.WithRequiredStructEnabled())
}

func TestPostExportRequest_HappyPath(t *testing.T) {
	t.Parallel()
	v := newValidator()
	req := dto.PostExportRequest{
		Entity:     "products",
		Format:     "ndjson",
		SnapshotID: uuid.New(),
	}
	require.NoError(t, v.Struct(req))
}

func TestPostExportRequest_BadFormat(t *testing.T) {
	t.Parallel()
	v := newValidator()
	req := dto.PostExportRequest{
		Entity:     "products",
		Format:     "csv", // not allowed
		SnapshotID: uuid.New(),
	}
	err := v.Struct(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Format")
}

func TestPostExportRequest_BadEntity(t *testing.T) {
	t.Parallel()
	v := newValidator()
	req := dto.PostExportRequest{
		Entity:     "totally_fake_entity",
		Format:     "ndjson",
		SnapshotID: uuid.New(),
	}
	err := v.Struct(req)
	require.Error(t, err)
}

func TestPageRequest_LimitTooLarge(t *testing.T) {
	t.Parallel()
	v := newValidator()
	req := dto.PageRequest{Cursor: "", Limit: 99999}
	err := v.Struct(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Limit")
}

func TestPageRequest_LimitOK(t *testing.T) {
	t.Parallel()
	v := newValidator()
	req := dto.PageRequest{Cursor: "", Limit: 100}
	require.NoError(t, v.Struct(req))
}

func TestFactsPageRequest_RequiresEventDate(t *testing.T) {
	t.Parallel()
	v := newValidator()
	req := dto.FactsPageRequest{Limit: 10}
	err := v.Struct(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "EventDate")
}
