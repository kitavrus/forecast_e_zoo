package repository

import (
	"context"
	"fmt"

	"github.com/Kitavrus/e_zoo/internal/features/orders/sqls/queries"
)

// NextSequence — выдаёт следующее значение orders.po_number_seq.
//
// Используется PONumberGenerator (фаза 4) — здесь только тонкая обёртка
// над pgx (можно брать как из pool, так и из tx через pgxpool.Pool / pgx.Tx).
func (r *Repository) NextSequence(ctx context.Context) (int64, error) {
	var seq int64
	err := r.pool.QueryRow(ctx, queries.MustGet("next_po_number_seq")).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("orders: nextval po_number_seq: %w", err)
	}
	return seq, nil
}
