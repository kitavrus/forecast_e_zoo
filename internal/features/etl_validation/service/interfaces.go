// Package service содержит orchestration-логику ETL pipeline:
// EtlPipeline (Extract → Stage → Validate → Transform → Load → Flip),
// EtlRunService (admin CRUD-API) и MartRefresh (ondemand-refresh).
package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/extractor"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/loader"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/transformer"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
)

// Repo — узкий контракт к repository, чтобы тесты могли подменять.
type Repo interface {
	InsertEtlRun(ctx context.Context, run *models.EtlRun) error
	GetEtlRunByID(ctx context.Context, id uuid.UUID) (*models.EtlRun, error)
	ListEtlRuns(ctx context.Context, f repository.EtlRunListFilter) ([]models.EtlRun, error)
	UpdateEtlRunStatus(ctx context.Context, id uuid.UUID, p repository.EtlRunStatusPatch) error
	UpdateEtlRunStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, p repository.EtlRunStatusPatch) error
	GetCurrentRunningEtlRun(ctx context.Context) (*models.EtlRun, error)
	InsertRejectEntries(ctx context.Context, entries []models.RejectLogEntry) (int64, error)
	ListRejectEntries(ctx context.Context, f repository.RejectLogListFilter) ([]models.RejectLogEntry, error)
	CreateStagingTables(ctx context.Context, tx pgx.Tx) error
	TryAdvisoryXactLock(ctx context.Context, tx pgx.Tx, key int64) (bool, error)
}

// Pool — минимальный интерфейс к pgxpool для service-слоя.
type Pool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// Extractor — узкий интерфейс для тестов.
type Extractor interface {
	GetCurrentSnapshot(ctx context.Context) (extractor.Snapshot, error)
	StreamEntity(ctx context.Context, entity, snapshotID, etag string) (extractor.NDJSONReader, error)
}

// ValidationEngine — узкий интерфейс к validation.Engine.
type ValidationEngine interface {
	Run(ds *validation.Dataset) validation.Report
}

// LoaderIface — узкий интерфейс к loader.Loader.
type LoaderIface interface {
	Apply(ctx context.Context, p loader.ApplyParams) (loader.BuildSummary, error)
}

// Registry — узкий интерфейс к transformer.Registry.
type Registry interface {
	BuildersForFullRun() []transformer.Builder
	BuilderByName(name string) (transformer.Builder, error)
}

// Metrics — узкий интерфейс к prometheus-метрикам.
//
// Реальная реализация — в фазе 16; здесь — тестируемая абстракция,
// которая может быть no-op для тестов.
type Metrics interface {
	RecordRunSuccess(durationSeconds float64)
	RecordRunFailure(durationSeconds float64, reason string)
	RecordRowsProcessed(mart string, rows int64)
}

// NoopMetrics — пустая реализация Metrics для тестов / dev-режима.
type NoopMetrics struct{}

// RecordRunSuccess — no-op.
func (NoopMetrics) RecordRunSuccess(_ float64) {}

// RecordRunFailure — no-op.
func (NoopMetrics) RecordRunFailure(_ float64, _ string) {}

// RecordRowsProcessed — no-op.
func (NoopMetrics) RecordRowsProcessed(_ string, _ int64) {}
