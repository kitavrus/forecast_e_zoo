package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Cursor — opaque page cursor для NDJSON-стримов и /v1/* пагинации.
//
// Сериализация: base64.URLEncoding(json{LoadID, AfterPK}).
// LoadID гарантирует читателю, что страница относится к тому же snapshot,
// AfterPK — sortable ключ "первичного ключа последней отданной строки"
// (например, "<event_date>|<receipt_id>|<line_no>" для receipt_line).
type Cursor struct {
	LoadID  string `json:"l"`
	AfterPK string `json:"k"`
}

// Encode сериализует cursor в base64 (URL-safe, no padding).
func (c Cursor) Encode() (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("cursor: marshal: %w", err)
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
		return fmt.Errorf("cursor: bad base64: %w", err)
	}
	if err := json.Unmarshal(raw, c); err != nil {
		return fmt.Errorf("cursor: bad json: %w", err)
	}
	return nil
}
