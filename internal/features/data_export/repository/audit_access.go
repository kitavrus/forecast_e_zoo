package repository

import (
	"context"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// AuditAccessInput — payload для аудита /admin/* запросов.
type AuditAccessInput struct {
	ActorRole string
	ActorSub  string
	Method    string
	Path      string
	Status    int
	TraceID   string
}

// InsertAudit пишет одну строку в audit_access.
func (r *Repository) InsertAudit(ctx context.Context, in AuditAccessInput) error {
	_, err := r.pool.Exec(ctx, queries.Get("audit_access_insert"),
		in.ActorRole, in.ActorSub, in.Method, in.Path, in.Status, in.TraceID)
	return mapError(err)
}
