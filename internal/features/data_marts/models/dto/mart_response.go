// Package dto содержит request/response DTO фичи data_marts.
package dto

import (
	"time"

	"github.com/google/uuid"
)

// MartInfoItem — один mart в листинге GET /v1/marts.
type MartInfoItem struct {
	Name        string    `json:"name"`
	EtlRunID    uuid.UUID `json:"etl_run_id,omitempty"`
	CommittedAt time.Time `json:"committed_at,omitempty"`
	Populated   bool      `json:"populated"`
}

// ListMartsResponse — ответ GET /v1/marts.
type ListMartsResponse struct {
	Marts []MartInfoItem `json:"marts"`
}

// MartVersionResponse — ответ GET /v1/marts/:name/version.
type MartVersionResponse struct {
	Name        string    `json:"name"`
	EtlRunID    uuid.UUID `json:"etl_run_id"`
	CommittedAt time.Time `json:"committed_at"`
}

// MartFieldDTO — поле в schema response.
type MartFieldDTO struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// MartSchemaResponse — ответ GET /v1/marts/:name/schema.
type MartSchemaResponse struct {
	Name   string         `json:"name"`
	Fields []MartFieldDTO `json:"fields"`
}
