package handler_test

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/handler"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/repository"
)

// --- AdminLoads mocks ---

type fakeLoadsRepo struct {
	running     *models.Load
	getByIDLoad models.Load
	getByIDErr  error
	rejects     []repository.RejectRow
	getRunErr   error
}

func (f *fakeLoadsRepo) GetByID(_ context.Context, _ uuid.UUID) (models.Load, error) {
	return f.getByIDLoad, f.getByIDErr
}

func (f *fakeLoadsRepo) GetRunning(_ context.Context) (*models.Load, error) {
	return f.running, f.getRunErr
}

func (f *fakeLoadsRepo) SelectRejects(_ context.Context, _ repository.RejectFilter, _ string, _ int) ([]repository.RejectRow, error) {
	return f.rejects, nil
}

// fakeTrigger — конфигурируемый мок AdminLoadsTrigger.
//
// tryAcquired управляет результатом TryTrigger:
//   - true  → load «принят», ответ 202.
//   - false → lock занят, хендлер должен вернуть 409.
type fakeTrigger struct {
	tryAcquired bool
	tryErr      error
	tryCalls    atomic.Int32
	triggerOnce atomic.Int32
}

func (f *fakeTrigger) TriggerOnce(_ context.Context) error {
	f.triggerOnce.Add(1)
	return nil
}

func (f *fakeTrigger) TryTrigger(_ context.Context) (bool, error) {
	f.tryCalls.Add(1)
	return f.tryAcquired, f.tryErr
}

// --- tests ---

func TestPostLoads_HappyPath_Returns202(t *testing.T) {
	t.Parallel()
	app := newApp()
	repo := &fakeLoadsRepo{} // нет running
	trig := &fakeTrigger{tryAcquired: true}
	h := handler.NewAdminLoadsHandler(repo, trig, repo)
	app.Post("/admin/loads", h.PostLoads)

	resp, err := app.Test(httptest.NewRequest("POST", "/admin/loads", strings.NewReader("{}")))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusAccepted, resp.StatusCode)
	require.Equal(t, int32(1), trig.tryCalls.Load())
}

func TestPostLoads_AlreadyRunning_Returns409WithCurrentLoadID(t *testing.T) {
	t.Parallel()
	app := newApp()
	runID := uuid.New()
	repo := &fakeLoadsRepo{running: &models.Load{ID: runID, Status: models.LoadStatusRunning}}
	trig := &fakeTrigger{tryAcquired: false} // не должен быть вызван — уже есть running
	h := handler.NewAdminLoadsHandler(repo, trig, repo)
	app.Post("/admin/loads", h.PostLoads)

	resp, err := app.Test(httptest.NewRequest("POST", "/admin/loads", strings.NewReader("{}")))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusConflict, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	require.Contains(t, bodyStr, "load_already_running")
	// currentLoadId = runID должен быть в details
	require.Contains(t, bodyStr, runID.String())
	require.Contains(t, bodyStr, "currentLoadId")

	// TryTrigger не должен вызываться, если уже есть running.
	require.Equal(t, int32(0), trig.tryCalls.Load())
}

// TestPostLoads_LockBusy_Returns409 — ключевой тест Issue #6:
// repo.GetRunning возвращает nil (load failed/завершился очень быстро),
// но advisory lock занят другим tick-ом → handler должен вернуть 409,
// а НЕ ложный 202.
func TestPostLoads_LockBusy_Returns409(t *testing.T) {
	t.Parallel()
	app := newApp()
	repo := &fakeLoadsRepo{} // нет running
	trig := &fakeTrigger{tryAcquired: false} // lock занят другим tick-ом
	h := handler.NewAdminLoadsHandler(repo, trig, repo)
	app.Post("/admin/loads", h.PostLoads)

	resp, err := app.Test(httptest.NewRequest("POST", "/admin/loads", strings.NewReader("{}")))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusConflict, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "load_already_running")
	// TryTrigger вызывался ровно один раз.
	require.Equal(t, int32(1), trig.tryCalls.Load())
	// TriggerOnce не вызывался — handler PostLoads больше его не использует.
	require.Equal(t, int32(0), trig.triggerOnce.Load())
}
