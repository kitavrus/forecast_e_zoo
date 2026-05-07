package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/sqls/queries"
)

// ReceiptLineRow — выборочная row для select_receipt_line (matches actual schema).
type ReceiptLineRow struct {
	ID         int64
	ReceiptID  string
	LocationID string
	ProductID  string
	Qty        float64
	Price      float64
	EventTime  time.Time
	EventDate  time.Time
	Payload    []byte
	LoadID     *uuid.UUID
}

// SelectReceiptLine — page с фильтром event_date BETWEEN dateFrom AND dateTo.
func (r *Repository) SelectReceiptLine(ctx context.Context, loadID uuid.UUID, afterPK string, limit int, dateFrom, dateTo time.Time) ([]ReceiptLineRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.pool.Query(ctx, queries.Get("select_receipt_line"),
		loadID, afterPK, limit, dateFrom, dateTo)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]ReceiptLineRow, 0, limit)
	for rows.Next() {
		var rl ReceiptLineRow
		if err := rows.Scan(
			&rl.ID, &rl.ReceiptID, &rl.LocationID, &rl.ProductID,
			&rl.Qty, &rl.Price, &rl.EventTime, &rl.EventDate,
			&rl.Payload, &rl.LoadID,
		); err != nil {
			return nil, mapError(err)
		}
		out = append(out, rl)
	}
	return out, mapError(rows.Err())
}

// InsertReceiptLineBatch вставляет batch строк через единый INSERT (используется loader-ом).
// Партиционирование по event_date — postgres сам маршрутизирует в нужную child-таблицу.
func (r *Repository) InsertReceiptLineBatch(ctx context.Context, tx pgx.Tx, batch []ReceiptLineRow, loadID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("InsertReceiptLineBatch requires tx")
	}
	if len(batch) == 0 {
		return nil
	}
	const sql = `
INSERT INTO receipt_line (id, receipt_id, location_id, product_id, qty, price,
                          event_time, event_date, payload, load_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10)
`
	for _, rl := range batch {
		if rl.Payload == nil {
			rl.Payload = []byte("{}")
		}
		if _, err := tx.Exec(ctx, sql,
			rl.ID, rl.ReceiptID, rl.LocationID, rl.ProductID, rl.Qty, rl.Price,
			rl.EventTime, rl.EventDate, rl.Payload, loadID,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

// ExplainSelectReceiptLine — для теста partition pruning. Возвращает строку плана.
func (r *Repository) ExplainSelectReceiptLine(ctx context.Context, loadID uuid.UUID, dateFrom, dateTo time.Time) (string, error) {
	rows, err := r.pool.Query(ctx,
		"EXPLAIN "+queries.Get("select_receipt_line"),
		loadID, "", 100, dateFrom, dateTo)
	if err != nil {
		return "", mapError(err)
	}
	defer rows.Close()
	var sb string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", mapError(err)
		}
		sb += line + "\n"
	}
	return sb, mapError(rows.Err())
}
