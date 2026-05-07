package validators_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler/validators"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

func TestParseCursor_Valid(t *testing.T) {
	t.Parallel()
	in := models.Cursor{LoadID: "L-1", AfterPK: "K-1"}
	enc, err := in.Encode()
	require.NoError(t, err)

	out, err := validators.ParseCursor(enc)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestParseCursor_EmptyOK(t *testing.T) {
	t.Parallel()
	out, err := validators.ParseCursor("")
	require.NoError(t, err)
	require.Equal(t, models.Cursor{}, out)
}

func TestParseCursor_Invalid_ReturnsErrInvalidCursor(t *testing.T) {
	t.Parallel()
	_, err := validators.ParseCursor("@@@not-base64@@@")
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrInvalidCursor))
}

func TestParseEventDateRange_FromGreaterThanTo_ReturnsErrInvalidQuery(t *testing.T) {
	t.Parallel()
	_, _, err := validators.ParseEventDateRange("2026-05-10", "2026-05-01")
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrInvalidQuery))
}

func TestParseEventDateRange_BadFormat_ReturnsErrInvalidQuery(t *testing.T) {
	t.Parallel()
	_, _, err := validators.ParseEventDateRange("not-a-date", "2026-05-01")
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrInvalidQuery))
}

func TestParseEventDateRange_OK(t *testing.T) {
	t.Parallel()
	f, to, err := validators.ParseEventDateRange("2026-05-01", "2026-05-07")
	require.NoError(t, err)
	require.True(t, !f.After(to))
}

func TestParseLimit_Negative_ReturnsErrBadRequest(t *testing.T) {
	t.Parallel()
	_, err := validators.ParseLimit("-5", 100, 1000)
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrBadRequest))
}

func TestParseLimit_NotInt_ReturnsErrBadRequest(t *testing.T) {
	t.Parallel()
	_, err := validators.ParseLimit("abc", 100, 1000)
	require.Error(t, err)
	require.True(t, errors.Is(err, errorspkg.ErrBadRequest))
}

func TestParseLimit_ClampToMax(t *testing.T) {
	t.Parallel()
	got, err := validators.ParseLimit("99999", 100, 1000)
	require.NoError(t, err)
	require.Equal(t, 1000, got)
}

func TestParseLimit_Default(t *testing.T) {
	t.Parallel()
	got, err := validators.ParseLimit("", 250, 1000)
	require.NoError(t, err)
	require.Equal(t, 250, got)
}

func TestNewValidator_RejectsBadFormat(t *testing.T) {
	t.Parallel()
	v := validators.NewValidator()
	req := dto.PostExportRequest{
		Entity:     "products",
		Format:     "xml", // not in oneof
		SnapshotID: uuid.New(),
	}
	require.Error(t, v.Struct(req))
}

func TestNewValidator_AcceptsHappyPath(t *testing.T) {
	t.Parallel()
	v := validators.NewValidator()
	req := dto.PostExportRequest{
		Entity:     "receipt_line",
		Format:     "ndjson",
		SnapshotID: uuid.New(),
	}
	require.NoError(t, v.Struct(req))
}
