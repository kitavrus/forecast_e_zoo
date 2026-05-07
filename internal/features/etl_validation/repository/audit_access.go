package repository

import (
	"context"
	"fmt"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/sqls/queries"
)

// InsertAuditAccess сохраняет запись marts.audit_access.
func (r *Repository) InsertAuditAccess(ctx context.Context, e models.AuditAccessEntry) error {
	exec := r.chooseExec(nil)
	_, err := exec.Exec(ctx, queries.MustGet("audit_access_insert"),
		e.Method, e.Path, e.Requester, e.Role, e.StatusCode, e.RequestID,
	)
	if err != nil {
		return fmt.Errorf("repository: InsertAuditAccess: %w", err)
	}
	return nil
}
