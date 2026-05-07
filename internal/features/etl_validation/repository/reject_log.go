package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/sqls/queries"
)

// InsertRejectEntries вставляет batch отвергнутых строк через pgx.CopyFrom.
//
// При больших объёмах (>1k строк) используется CopyFrom — оптимальнее
// многократного INSERT.
func (r *Repository) InsertRejectEntries(ctx context.Context, entries []models.RejectLogEntry) (int64, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	rows := make([][]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []any{
			e.EtlRunID,
			e.Entity,
			e.BusinessKey,
			e.Severity,
			e.RuleID,
			e.Field,
			e.Message,
		})
	}
	cols := []string{"etl_run_id", "entity", "business_key", "severity", "rule_id", "field", "message"}
	n, err := r.pool.CopyFrom(ctx, pgx.Identifier{"marts", "reject_log"}, cols, pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("repository: InsertRejectEntries CopyFrom: %w", err)
	}
	return n, nil
}

// RejectLogListFilter — фильтр для cursor-pagination по reject_log.
type RejectLogListFilter struct {
	EtlRunID *uuid.UUID
	Entity   string
	Severity string
	BeforeID *int64
	Limit    int
}

// ListRejectEntries возвращает страницу reject_log.
func (r *Repository) ListRejectEntries(ctx context.Context, f RejectLogListFilter) ([]models.RejectLogEntry, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	var runArg any
	if f.EtlRunID != nil {
		runArg = *f.EtlRunID
	}
	var entityArg, severityArg any
	if f.Entity != "" {
		entityArg = f.Entity
	}
	if f.Severity != "" {
		severityArg = f.Severity
	}
	var beforeArg any
	if f.BeforeID != nil {
		beforeArg = *f.BeforeID
	}
	exec := r.chooseExec(nil)
	rows, err := exec.Query(ctx, queries.MustGet("reject_log_list"),
		runArg, entityArg, severityArg, beforeArg, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("repository: ListRejectEntries: %w", err)
	}
	defer rows.Close()

	out := make([]models.RejectLogEntry, 0, limit)
	for rows.Next() {
		var e models.RejectLogEntry
		scanErr := rows.Scan(
			&e.ID, &e.EtlRunID, &e.Entity, &e.BusinessKey,
			&e.Severity, &e.RuleID, &e.Field, &e.Message, &e.CreatedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("repository: ListRejectEntries scan: %w", scanErr)
		}
		out = append(out, e)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, fmt.Errorf("repository: ListRejectEntries rows.Err: %w", rerr)
	}
	return out, nil
}
