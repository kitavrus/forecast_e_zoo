package handler

import (
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/models/dto"
)

func toEtlRunResponse(r *models.EtlRun) dto.EtlRunResponse {
	out := dto.EtlRunResponse{
		ID:            r.ID.String(),
		StartedAt:     r.StartedAt,
		FinishedAt:    r.FinishedAt,
		CommittedAt:   r.CommittedAt,
		Status:        r.Status,
		Kind:          r.Kind,
		TargetMart:    r.TargetMart,
		Trigger:       r.Trigger,
		Requester:     r.Requester,
		MartsSummary:  r.MartsSummary,
		FailureReason: r.FailureReason,
		LinesTotal:    r.LinesTotal,
		LinesFailed:   r.LinesFailed,
	}
	if r.SourceLoadID != nil {
		s := r.SourceLoadID.String()
		out.SourceLoadID = &s
	}
	if r.ParentRunID != nil {
		s := r.ParentRunID.String()
		out.ParentRunID = &s
	}
	return out
}

func toRejectLogResponse(e models.RejectLogEntry) dto.RejectLogEntryResponse {
	return dto.RejectLogEntryResponse{
		ID:          e.ID,
		EtlRunID:    e.EtlRunID.String(),
		Entity:      e.Entity,
		BusinessKey: e.BusinessKey,
		Severity:    e.Severity,
		RuleID:      e.RuleID,
		Field:       e.Field,
		Message:     e.Message,
		CreatedAt:   e.CreatedAt,
	}
}
