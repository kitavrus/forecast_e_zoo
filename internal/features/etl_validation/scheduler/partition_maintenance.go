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

// PartitionMaintainer обеспечивает наличие партиций для текущего и следующего месяца.
type PartitionMaintainer interface {
	EnsureNextMonth(ctx context.Context) error
}

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

// EnsureNextMonth создаёт партиции для текущего и следующего месяца, если их нет.
//
// Имена партиций — `<base>_<YYYY>_<MM>`, диапазон — [first day, first day next month).
func (m *partitionMaintainer) EnsureNextMonth(ctx context.Context) error {
	now := m.now()
	for _, base := range m.tables {
		for monthShift := 0; monthShift <= 1; monthShift++ {
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
