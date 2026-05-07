package scheduler_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/scheduler"
)

func TestScheduler_New_InvalidTZ(t *testing.T) {
	t.Parallel()
	_, err := scheduler.New(scheduler.Config{TZ: "Mars/Olympus"}, nil, nil, nil)
	require.Error(t, err)
}

func TestScheduler_Start_EmptyCronExpr(t *testing.T) {
	t.Parallel()
	s, err := scheduler.New(scheduler.Config{TZ: "UTC"}, nil, nil, nil)
	require.NoError(t, err)
	defer func() { _ = s.Stop() }()

	err = s.Start(nil) //nolint:staticcheck // ctx не используется в Start
	require.Error(t, err)
}
