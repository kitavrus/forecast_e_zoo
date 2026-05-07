package scheduler_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/orders/scheduler"
)

func TestScheduler_New_InvalidTZ(t *testing.T) {
	t.Parallel()
	_, err := scheduler.New(scheduler.Config{
		CronExpr: "0 6 * * *",
		TZ:       "Invalid/TZ",
	}, nil, nil, nil)
	require.Error(t, err)
}

func TestScheduler_New_DefaultTimeoutAndMaxPlans(t *testing.T) {
	t.Parallel()
	s, err := scheduler.New(scheduler.Config{
		CronExpr: "0 6 * * *",
		TZ:       "UTC",
	}, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, s)
}

func TestScheduler_Start_EmptyCron_Errors(t *testing.T) {
	t.Parallel()
	s, err := scheduler.New(scheduler.Config{
		CronExpr: "",
		TZ:       "UTC",
	}, nil, nil, nil)
	require.NoError(t, err)
	require.Error(t, s.Start(nil))
}
