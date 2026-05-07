package observability_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/observability"
)

func TestMetrics_AreRegistered(t *testing.T) {
	t.Parallel()
	reg := observability.Init()
	require.NotNil(t, reg)
	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.NotEmpty(t, mfs)
}

func TestMetrics_LoadSuccessIncrements(t *testing.T) {
	t.Parallel()
	c := observability.LoadSuccessTotal.WithLabelValues("erp_e_zoo")
	before := testutil.ToFloat64(c)
	c.Inc()
	c.Inc()
	after := testutil.ToFloat64(c)
	require.InDelta(t, before+2, after, 0.0001)
}

func TestMetrics_HTTPHandlerExposesMetrics(t *testing.T) {
	t.Parallel()
	reg := observability.Init()
	app := fiber.New()
	app.Get("/metrics", observability.Handler(reg))

	// trigger one counter so metrics file isn't empty
	observability.SnapshotNotReadyTotal.Inc()

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	require.True(t,
		strings.Contains(out, "source_adapter_") ||
			strings.Contains(out, "# HELP") ||
			strings.Contains(out, "TYPE"),
		"expected metrics output, got: %s", out)
}
