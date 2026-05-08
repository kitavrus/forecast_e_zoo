// Package scheduler реализует периодический запуск ETL pipeline через gocron
// с предварительной partition maintenance для mart_demand_history и mart_kpi_daily.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
)

// PartitionMaintainer обеспечивает наличие партиций для исторического окна
// (last factsHistoryDays месяцев) и следующего месяца — синхронно с windows
// ETL extractor (см. service/staging.go::factsHistoryDays = 365) и
// source-adapter pull (см. internal/config.LoadFactsWindowDays).
type PartitionMaintainer interface {
	EnsureNextMonth(ctx context.Context) error
}

// historyMonthsBack — глубина исторического окна партиций (12 месяцев).
// Совпадает с factsHistoryDays / LoadFactsWindowDays (≈365 дней).
// Без этого партиции покрывают только current+next month, и upsert
// в mart_demand_history падает с "no partition of relation found"
// для исторических event_date (баг 2026-05-08).
const historyMonthsBack = 12

// partitionMaintainer реализует DDL CREATE TABLE IF NOT EXISTS PARTITION OF.
type partitionMaintainer struct {
	pool   *pgxpool.Pool
	tables []string
	now    func() time.Time
}

// NewPartitionMaintainer — DI-конструктор. По умолчанию maintain-ит
// mart_demand_history и mart_kpi_daily.
func NewPartitionMaintainer(pool *pgxpool.Pool) PartitionMaintainer {
	return &partitionMaintainer{
		pool:   pool,
		tables: []string{constants.MartDemandHistory, constants.MartKpiDaily},
		now:    time.Now,
	}
}

// EnsureNextMonth создаёт партиции для исторического окна (last
// historyMonthsBack месяцев) + текущего + следующего месяца, если их нет.
//
// Имена партиций — `<base>_<YYYY>_<MM>`, диапазон — [first day, first day next month).
// Идемпотентно (CREATE TABLE IF NOT EXISTS).
func (m *partitionMaintainer) EnsureNextMonth(ctx context.Context) error {
	now := m.now()
	for _, base := range m.tables {
		for monthShift := -historyMonthsBack; monthShift <= 1; monthShift++ {
			start := time.Date(now.Year(), now.Month()+time.Month(monthShift), 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, 0)
			suffix := fmt.Sprintf("%04d_%02d", start.Year(), int(start.Month()))
			partName := fmt.Sprintf("marts.%s_%s", base, suffix)
			ddl := fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s PARTITION OF marts.%s FOR VALUES FROM ('%s') TO ('%s');",
				partName, base, start.Format("2006-01-02"), end.Format("2006-01-02"),
			)
			if _, err := m.pool.Exec(ctx, ddl); err != nil {
				return fmt.Errorf("partition_maintenance: create %s: %w", partName, err)
			}
		}
	}
	return nil
}

// NoopPartitionMaintainer — для тестов, в которых маинтенанс не нужен.
type NoopPartitionMaintainer struct{}

// EnsureNextMonth — no-op.
func (NoopPartitionMaintainer) EnsureNextMonth(_ context.Context) error { return nil }
