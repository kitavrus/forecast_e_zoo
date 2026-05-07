// Package formatter — pluggable BodyFormatter для channel-routing.
//
// MVP: JSON. Future hooks: EDI X12, EDIFACT, 1С XML.
package formatter

import (
	"errors"
	"fmt"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

// PurchaseOrderPayload — данные PO + lines, передаются в BodyFormatter.
//
// MVP: только заголовок PO; PO lines подгружаются отдельным расширением.
// Структура зеркалит channels.models.PurchaseOrderForSend для простоты + Lines.
type PurchaseOrderPayload struct {
	PO    models.PurchaseOrderForSend
	Lines []POLine // optional; MVP может быть пустым
}

// POLine — одна строка PO для отправки. MVP-минимум.
type POLine struct {
	SKU      string  `json:"sku"`
	Qty      float64 `json:"qty"`
	Unit     string  `json:"unit"`
	UnitCost float64 `json:"unit_cost,omitempty"`
}

// Formatter — сериализация PO в body для конкретного канала.
type Formatter interface {
	// Format возвращает сериализованный body + content-type для HTTP-заголовка.
	Format(payload PurchaseOrderPayload) (body []byte, contentType string, err error)

	// ChannelType возвращает supported channel_type (erp_api|edi_x12|...).
	ChannelType() string
}

// Registry — выбирает Formatter по channel_type.
type Registry struct {
	formatters map[string]Formatter
}

// NewRegistry собирает реестр.
func NewRegistry(formatters ...Formatter) *Registry {
	m := make(map[string]Formatter, len(formatters))
	for _, f := range formatters {
		if f == nil {
			continue
		}
		m[f.ChannelType()] = f
	}
	return &Registry{formatters: m}
}

// Get возвращает formatter или error.
func (r *Registry) Get(channelType string) (Formatter, error) {
	if r == nil {
		return nil, errors.New("formatter: registry is nil")
	}
	f, ok := r.formatters[channelType]
	if !ok {
		return nil, fmt.Errorf("formatter: channel %q is not supported", channelType)
	}
	return f, nil
}

// Compile-time assertion (использует constants).
var _ = constants.ChannelTypeErpAPI
