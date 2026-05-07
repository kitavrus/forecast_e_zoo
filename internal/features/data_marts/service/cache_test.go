package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
)

// TestVersionCache_HitMiss — get/put базовый поток.
func TestVersionCache_HitMiss(t *testing.T) {
	t.Parallel()
	c := newVersionCache(60 * time.Second)
	_, ok := c.get("mart_kpi_daily")
	assert.False(t, ok, "пустой cache → miss")

	v := models.MartVersion{Name: "mart_kpi_daily", EtlRunID: uuid.New(), CommittedAt: time.Now()}
	c.put("mart_kpi_daily", v)

	got, ok := c.get("mart_kpi_daily")
	require.True(t, ok)
	assert.Equal(t, v.EtlRunID, got.EtlRunID)
}

// TestVersionCache_TTLEviction — после TTL запись evict-ится.
func TestVersionCache_TTLEviction(t *testing.T) {
	t.Parallel()
	c := newVersionCache(100 * time.Millisecond)

	now := time.Now()
	c.now = func() time.Time { return now }

	v := models.MartVersion{Name: "x", EtlRunID: uuid.New()}
	c.put("x", v)

	// до истечения TTL.
	_, ok := c.get("x")
	assert.True(t, ok)

	// перематываем clock на 200ms вперёд.
	c.now = func() time.Time { return now.Add(200 * time.Millisecond) }

	_, ok = c.get("x")
	assert.False(t, ok, "после TTL — miss")
}

// TestVersionCache_Invalidate.
func TestVersionCache_Invalidate(t *testing.T) {
	t.Parallel()
	c := newVersionCache(60 * time.Second)
	c.put("a", models.MartVersion{Name: "a"})
	c.invalidate("a")
	_, ok := c.get("a")
	assert.False(t, ok)
}
