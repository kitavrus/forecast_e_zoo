package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/service"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// fakeReader — минимальный mock MartReader для тестов Service.
type fakeReader struct {
	listResp    []models.MartInfo
	listErr     error
	readRows    []models.MartRow
	readNext    string
	readVersion models.MartVersion
	readErr     error
	verResp     models.MartVersion
	verErr      error
	schemaResp  models.MartSchema
	schemaErr   error

	// counters для проверки делегации
	readCalls    int
	versionCalls int
}

func (f *fakeReader) List(_ context.Context) ([]models.MartInfo, error) {
	return f.listResp, f.listErr
}
func (f *fakeReader) Read(_ context.Context, _ string, _ string, _ int) (
	[]models.MartRow, string, models.MartVersion, error,
) {
	f.readCalls++
	return f.readRows, f.readNext, f.readVersion, f.readErr
}
func (f *fakeReader) GetVersion(_ context.Context, _ string) (models.MartVersion, error) {
	f.versionCalls++
	return f.verResp, f.verErr
}
func (f *fakeReader) GetSchema(_ context.Context, _ string) (models.MartSchema, error) {
	return f.schemaResp, f.schemaErr
}

func TestService_List_DelegatesToReader(t *testing.T) {
	t.Parallel()
	r := &fakeReader{listResp: []models.MartInfo{{Name: "mart_kpi_daily"}}}
	s := service.New(r)
	out, err := s.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "mart_kpi_daily", out[0].Name)
}

func TestService_GetVersion_PropagatesError(t *testing.T) {
	t.Parallel()
	r := &fakeReader{verErr: errorspkg.ErrServiceUnavailable}
	s := service.New(r)
	_, err := s.GetVersion(context.Background(), constants.MartKpiDaily)
	require.Error(t, err)
	assert.ErrorIs(t, err, errorspkg.ErrServiceUnavailable)
}

func TestService_Read_ReturnsCursor(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	r := &fakeReader{
		readRows:    []models.MartRow{{"k": "v"}},
		readNext:    "next-cursor-b64",
		readVersion: models.MartVersion{Name: "x", EtlRunID: id, CommittedAt: time.Now()},
	}
	s := service.New(r)
	rows, next, ver, err := s.Read(context.Background(), constants.MartKpiDaily, "", 100)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "next-cursor-b64", next)
	assert.Equal(t, id, ver.EtlRunID)
	assert.Equal(t, 1, r.readCalls)
}

func TestService_GetSchema_DelegatesAndReturnsFields(t *testing.T) {
	t.Parallel()
	r := &fakeReader{schemaResp: models.MartSchema{
		Name:   "mart_kpi_daily",
		Fields: []models.MartField{{Name: "kpi_value", Type: "numeric"}},
	}}
	s := service.New(r)
	sch, err := s.GetSchema(context.Background(), constants.MartKpiDaily)
	require.NoError(t, err)
	assert.Equal(t, "mart_kpi_daily", sch.Name)
	require.Len(t, sch.Fields, 1)
	assert.Equal(t, "kpi_value", sch.Fields[0].Name)
}
