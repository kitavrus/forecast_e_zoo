// Package models содержит domain types фичи data_marts.
package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MartInfo — описывает один mart с текущей версией для GET /v1/marts.
type MartInfo struct {
	Name        string    `json:"name"`
	EtlRunID    uuid.UUID `json:"etl_run_id"`
	CommittedAt time.Time `json:"committed_at"`
}

// MartVersion — версия mart'а (текущий committed etl_run_id) для GET /v1/marts/:name/version.
type MartVersion struct {
	Name        string    `json:"name"`
	EtlRunID    uuid.UUID `json:"etl_run_id"`
	CommittedAt time.Time `json:"committed_at"`
}

// MartField — описание одного поля в схеме mart'а.
type MartField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// MartSchema — схема mart'а для GET /v1/marts/:name/schema.
type MartSchema struct {
	Name   string      `json:"name"`
	Fields []MartField `json:"fields"`
}

// MartRow — одна строка mart'а для streaming.
// Используем map[string]any чтобы не плодить 5 разных DTO под каждый mart;
// клиент знает schema через GET /v1/marts/:name/schema.
type MartRow map[string]any

// Cursor — opaque page cursor для NDJSON-стримов /v1/marts/:name.
//
// Сериализация: base64.RawURLEncoding(json{EtlRunID, LastPK}).
// EtlRunID гарантирует читателю, что страница относится к тому же snapshot.
// LastPK — конкатенация PK-полей последней отданной строки (формат — see ADR-001).
type Cursor struct {
	EtlRunID uuid.UUID `json:"r"`
	LastPK   string    `json:"k"`
}

// Encode сериализует cursor в base64 URL-safe (без padding).
func (c Cursor) Encode() (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("data_marts cursor: marshal: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Decode парсит cursor из строки. Пустая строка → empty Cursor, без ошибки
// (это означает «начало стрима»).
func (c *Cursor) Decode(s string) error {
	if s == "" {
		*c = Cursor{}
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("data_marts cursor: bad base64: %w", err)
	}
	if err := json.Unmarshal(raw, c); err != nil {
		return fmt.Errorf("data_marts cursor: bad json: %w", err)
	}
	return nil
}
