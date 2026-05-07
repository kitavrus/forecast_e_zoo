// Package formatter — JSONFormatter (MVP, channel_type=erp_api).
package formatter

import (
	"encoding/json"
	"fmt"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
)

// JSONFormatter — MVP реализация для erp_api.
type JSONFormatter struct{}

// NewJSONFormatter создаёт formatter.
func NewJSONFormatter() *JSONFormatter { return &JSONFormatter{} }

// ChannelType возвращает constants.ChannelTypeErpAPI.
func (f *JSONFormatter) ChannelType() string { return constants.ChannelTypeErpAPI }

// jsonPOPayload — wire-формат PO в JSON для ERP клиента.
//
// Согласовывается с IT E-Zoo (см. ADR-010 в design.md). MVP — стабильная shape.
type jsonPOPayload struct {
	PONumber       string         `json:"po_number"`
	IdempotencyKey string         `json:"idempotency_key"`
	SupplierID     string         `json:"supplier_id"`
	LocationID     string         `json:"location_id"`
	TotalQty       float64        `json:"total_qty"`
	Currency       string         `json:"currency"`
	CreatedAt      string         `json:"created_at"`
	Lines          []jsonPOLine   `json:"lines,omitempty"`
}

type jsonPOLine struct {
	SKU      string  `json:"sku"`
	Qty      float64 `json:"qty"`
	Unit     string  `json:"unit"`
	UnitCost float64 `json:"unit_cost,omitempty"`
}

// Format сериализует PO в JSON.
func (f *JSONFormatter) Format(p PurchaseOrderPayload) ([]byte, string, error) {
	out := jsonPOPayload{
		PONumber:       p.PO.PONumber,
		IdempotencyKey: p.PO.PONumber, // ADR-006: idempotency key = po_number
		SupplierID:     p.PO.SupplierID,
		LocationID:     p.PO.LocationID,
		TotalQty:       p.PO.TotalQty,
		Currency:       p.PO.Currency,
		CreatedAt:      p.PO.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if len(p.Lines) > 0 {
		out.Lines = make([]jsonPOLine, 0, len(p.Lines))
		for _, l := range p.Lines {
			out.Lines = append(out.Lines, jsonPOLine{
				SKU: l.SKU, Qty: l.Qty, Unit: l.Unit, UnitCost: l.UnitCost,
			})
		}
	}
	body, err := json.Marshal(out)
	if err != nil {
		return nil, "", fmt.Errorf("formatter/json: marshal: %w", err)
	}
	return body, "application/json", nil
}

// NotImplementedFormatter — заглушка для edi_x12/edi_edifact/1c_xml/crm.
type NotImplementedFormatter struct{ Channel string }

// ChannelType возвращает заявленный channel_type.
func (f *NotImplementedFormatter) ChannelType() string { return f.Channel }

// Format всегда возвращает ошибку.
func (f *NotImplementedFormatter) Format(_ PurchaseOrderPayload) ([]byte, string, error) {
	return nil, "", fmt.Errorf("formatter/%s: not implemented in MVP", f.Channel)
}
