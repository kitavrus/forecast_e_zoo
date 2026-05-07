package handler

import (
	"strconv"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/repository"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validators"
)

func buildRejectFilter(q validators.ListRejectLogQuery) repository.RejectLogListFilter {
	f := repository.RejectLogListFilter{
		Entity:   q.Entity,
		Severity: q.Severity,
		Limit:    q.Limit,
	}
	if q.EtlRunID != "" {
		if uid, err := uuid.Parse(q.EtlRunID); err == nil {
			f.EtlRunID = &uid
		}
	}
	if q.Cursor != "" {
		if v, err := strconv.ParseInt(q.Cursor, 10, 64); err == nil {
			f.BeforeID = &v
		}
	}
	return f
}
