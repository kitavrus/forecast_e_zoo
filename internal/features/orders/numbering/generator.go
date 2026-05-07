// Package numbering — генератор PO номеров формата PO-YYYYMMDD-NNNNNN.
//
// Sequence-часть берётся из orders.po_number_seq (PG SEQUENCE), что
// гарантирует уникальность под параллельными INSERT-ами.
package numbering

import (
	"context"
	"fmt"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
)

// SeqProvider — источник чисел для номера. В prod-коде это repository.NextSequence,
// в тестах — обычная ин-мем счётчик.
type SeqProvider interface {
	NextSequence(ctx context.Context) (int64, error)
}

// Generator — формирует PO номер.
type Generator struct {
	provider SeqProvider
}

// New создаёт Generator.
func New(p SeqProvider) *Generator {
	return &Generator{provider: p}
}

// Format — собирает номер из date + sequence value (zero-padded width 6).
//
// Экспонируется как pure function для unit-тестов и stub-генерации.
func Format(date time.Time, seq int64) string {
	return fmt.Sprintf("PO-%s-%0*d",
		date.UTC().Format("20060102"),
		constants.PONumberSeqWidth,
		seq,
	)
}

// Next возвращает следующий номер на конкретную дату.
//
// Дата = today UTC если zero. SEQUENCE монотонная глобально, не resetиncs per day —
// это упрощает race conditions, ценой того, что seq-часть не "сбрасывается" в 000001
// каждый день (бизнес-приемлемо для MVP, ADR-001).
func (g *Generator) Next(ctx context.Context, date time.Time) (string, error) {
	if date.IsZero() {
		date = time.Now().UTC()
	}
	seq, err := g.provider.NextSequence(ctx)
	if err != nil {
		return "", fmt.Errorf("po number: get sequence: %w", err)
	}
	return Format(date, seq), nil
}
