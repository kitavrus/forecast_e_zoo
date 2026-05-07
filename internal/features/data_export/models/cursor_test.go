package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCursor_EncodeDecode_Roundtrip(t *testing.T) {
	t.Parallel()
	in := Cursor{LoadID: "11111111-2222-3333-4444-555555555555", AfterPK: "2026-05-07|R-001|3"}

	encoded, err := in.Encode()
	require.NoError(t, err)
	require.NotEmpty(t, encoded)
	require.False(t, strings.Contains(encoded, "="), "raw url encoding should have no padding")

	var out Cursor
	require.NoError(t, out.Decode(encoded))
	require.Equal(t, in, out)
}

func TestCursor_DecodeEmpty_Empty(t *testing.T) {
	t.Parallel()
	var c Cursor
	require.NoError(t, c.Decode(""))
	require.Equal(t, Cursor{}, c)
}

func TestCursor_DecodeBadBase64_Error(t *testing.T) {
	t.Parallel()
	var c Cursor
	err := c.Decode("!!!not_base64!!!")
	require.Error(t, err)
	require.Contains(t, err.Error(), "base64")
}

func TestCursor_DecodeBadJSON_Error(t *testing.T) {
	t.Parallel()
	// валидный base64, но не валидный JSON
	bad := "bm90LWpzb24" // base64.RawURLEncoding("not-json")
	var c Cursor
	err := c.Decode(bad)
	require.Error(t, err)
	require.Contains(t, err.Error(), "json")
}
