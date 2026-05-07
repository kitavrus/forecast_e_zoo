package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PartitionedTables — список parent-таблиц, разбитых по event_date.
var PartitionedTables = []string{
	"receipt_line",
	"location_stock_snapshot",
	"stock_movement",
	"supplier_stock_snapshot",
}

// MonthRange — диапазон одной месячной партиции.
type MonthRange struct {
	From time.Time // первый день месяца
	To   time.Time // первый день следующего месяца
}

// nextMonthRanges возвращает срез MonthRange длиной monthsAhead, начиная с текущего месяца now.
func nextMonthRanges(now time.Time, monthsAhead int) []MonthRange {
	out := make([]MonthRange, 0, monthsAhead+1)
	cur := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= monthsAhead; i++ {
		out = append(out, MonthRange{
			From: cur,
			To:   cur.AddDate(0, 1, 0),
		})
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

// childTableName формирует имя дочерней партиции: <parent>_yYYYYmMM.
func childTableName(parent string, t time.Time) string {
	return fmt.Sprintf("%s_y%04dm%02d", parent, t.Year(), int(t.Month()))
}

// partitionDDL формирует CREATE TABLE IF NOT EXISTS ... PARTITION OF ... statement.
func partitionDDL(parent string, r MonthRange) string {
	child := childTableName(parent, r.From)
	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
		child, parent,
		r.From.Format("2006-01-02"),
		r.To.Format("2006-01-02"),
	)
}

// EnsureNextPartitions идемпотентно создаёт месячные партиции для всех 4
// partitioned-таблиц на текущий + monthsAhead месяцев вперёд.
func EnsureNextPartitions(ctx context.Context, pool *pgxpool.Pool, now time.Time, monthsAhead int) error {
	ranges := nextMonthRanges(now, monthsAhead)
	for _, table := range PartitionedTables {
		for _, r := range ranges {
			ddl := partitionDDL(table, r)
			if _, err := pool.Exec(ctx, ddl); err != nil {
				return fmt.Errorf("scheduler: ensure partition %s: %w", childTableName(table, r.From), err)
			}
		}
	}
	return nil
}
