// Package sender — pluggable ChannelSender для channel-routing (Module 7).
//
// MVP: только ErpAPISender (REST API клиент). Future: EDI/1С/CRM.
package sender

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/channels/formatter"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

// SendResult — результат одной попытки отправки.
type SendResult struct {
	ExternalRef    *string    // ID/номер в external system; nil если не получили
	Status         string     // pending|success|failed|skipped (см. constants.SendAttemptStatus*)
	HTTPStatusCode *int       // nil если до HTTP не дошло (network err)
	RequestBody    *string    // для audit
	ResponseBody   *string    // для audit (truncated)
	ErrorMessage   *string    // для audit/troubleshooting
	RetryCount     int        // количество фактических retry внутри Send
	StartedAt      time.Time  // фактическое время старта (для PRIMARY KEY partition)
	FinishedAt     *time.Time // время завершения; nil только если status='pending' (не должно быть)
}

// SendInput — входные данные для Send.
type SendInput struct {
	PO    models.PurchaseOrderForSend
	Lines []formatter.POLine // optional MVP
}

// ChannelSender — интерфейс отправки одного PO в один канал.
//
// Реализации: ErpAPISender (MVP), позже EdiX12Sender, OneCXMLSender, CRMSender.
type ChannelSender interface {
	// Send выполняет одну логическую отправку (с внутренним retry/backoff).
	Send(ctx context.Context, in SendInput, cfg models.SupplierChannelConfig) (SendResult, error)

	// ChannelType возвращает поддерживаемый channel_type (constants.ChannelType*).
	ChannelType() string
}

// Registry — выбирает ChannelSender по channel_type.
type Registry struct {
	senders map[string]ChannelSender
}

// NewRegistry собирает реестр.
func NewRegistry(senders ...ChannelSender) *Registry {
	m := make(map[string]ChannelSender, len(senders))
	for _, s := range senders {
		if s == nil {
			continue
		}
		m[s.ChannelType()] = s
	}
	return &Registry{senders: m}
}

// Get возвращает sender или error.
func (r *Registry) Get(channelType string) (ChannelSender, error) {
	if r == nil {
		return nil, errors.New("sender: registry is nil")
	}
	s, ok := r.senders[channelType]
	if !ok {
		return nil, fmt.Errorf("sender: channel %q is not supported", channelType)
	}
	return s, nil
}

// Names возвращает все зарегистрированные channel_type (для диагностики).
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.senders))
	for k := range r.senders {
		out = append(out, k)
	}
	return out
}

// NotImplementedSender — заглушка для edi_x12/edi_edifact/1c_xml/crm.
type NotImplementedSender struct{ Channel string }

// ChannelType возвращает заявленный channel.
func (s *NotImplementedSender) ChannelType() string { return s.Channel }

// Send всегда возвращает ошибку.
func (s *NotImplementedSender) Send(
	_ context.Context, _ SendInput, _ models.SupplierChannelConfig,
) (SendResult, error) {
	return SendResult{
		Status:    "failed",
		StartedAt: time.Now().UTC(),
	}, fmt.Errorf("sender/%s: not implemented in MVP", s.Channel)
}

// ptrStr возвращает *string или nil если строка пустая.
func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ptrInt возвращает *int.
func ptrInt(v int) *int { return &v }

// noopUUID — для MVP возможной диагностики (не используется в логике).
var _ = uuid.Nil
