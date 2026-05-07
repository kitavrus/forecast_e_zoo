package numbering_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/orders/numbering"
)

type stubProvider struct {
	counter atomic.Int64
	err     error
}

func (s *stubProvider) NextSequence(_ context.Context) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.counter.Add(1), nil
}

func TestFormat_PaddingAndDate(t *testing.T) {
	t.Parallel()
	date := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "PO-20260507-000001", numbering.Format(date, 1))
	assert.Equal(t, "PO-20260507-000042", numbering.Format(date, 42))
	assert.Equal(t, "PO-20260507-123456", numbering.Format(date, 123456))
}

func TestFormat_MillionPlus(t *testing.T) {
	t.Parallel()
	date := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	// если seq > 999999, padding не "обрезает" — просто шире.
	assert.Equal(t, "PO-20261231-1000000", numbering.Format(date, 1_000_000))
}

func TestNext_HappyPath(t *testing.T) {
	t.Parallel()
	p := &stubProvider{}
	g := numbering.New(p)
	got, err := g.Next(context.Background(), time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, "PO-20260507-000001", got)

	got2, err := g.Next(context.Background(), time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, "PO-20260507-000002", got2)
}

func TestNext_ZeroDateUsesNow(t *testing.T) {
	t.Parallel()
	p := &stubProvider{}
	g := numbering.New(p)
	got, err := g.Next(context.Background(), time.Time{})
	require.NoError(t, err)
	// формат: PO-YYYYMMDD-000001
	require.Len(t, got, len("PO-20260507-000001"))
	assert.Equal(t, "PO-", got[:3])
	assert.Equal(t, "-000001", got[len(got)-7:])
}

func TestNext_ProviderError(t *testing.T) {
	t.Parallel()
	p := &stubProvider{err: errors.New("db down")}
	g := numbering.New(p)
	_, err := g.Next(context.Background(), time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC))
	require.Error(t, err)
}
