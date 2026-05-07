package service

import (
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
)

// violationsToRejectEntries — мапит validation.Violation → models.RejectLogEntry
// для пакетного INSERT в marts.reject_log.
//
// Формат BusinessKey/Field — pointer-string чтобы корректно представить NULL
// в БД (для нарушений без бизнес-ключа или без конкретного поля).
func violationsToRejectEntries(runID uuid.UUID, vs []validation.Violation) []models.RejectLogEntry {
	out := make([]models.RejectLogEntry, 0, len(vs))
	for _, v := range vs {
		entry := models.RejectLogEntry{
			EtlRunID: runID,
			Entity:   v.Entity,
			Severity: string(v.Severity),
			RuleID:   v.RuleName,
			Message:  v.Message,
		}
		if v.BusinessKey != "" {
			bk := v.BusinessKey
			entry.BusinessKey = &bk
		}
		if v.Field != "" {
			f := v.Field
			entry.Field = &f
		}
		out = append(out, entry)
	}
	return out
}
