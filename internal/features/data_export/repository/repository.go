// Package repository — pgx/v5 + pgxpool слой доступа к БД.
//
// Контракты:
//   - все методы (ctx, ...) (..., error);
//   - sentinel-ошибки маппятся через pkg/errorspkg
//     (pgx.ErrNoRows → ErrNotFound; pgconn 23505 → ErrAlreadyExists);
//   - SQL-запросы — go:embed из internal/.../sqls/queries (фаза 06).
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Repository — корневой struct, хранит pool и ничего больше.
// Транзакции принимаются как параметр (pgx.Tx) в методы upsert/flip.
type Repository struct {
	pool *pgxpool.Pool
}

// New создаёт Repository поверх готового pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool возвращает pool — для loader-а (BeginTx).
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// mapError — конвертирует pgx-ошибку в sentinel из errorspkg.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errorspkg.ErrNotFound.Wrap(err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			return errorspkg.ErrAlreadyExists.Wrap(err)
		}
	}
	return err
}

// txOrPool — внутренний интерфейс «исполнителя запроса», единый для tx и pool.
type queryExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// chooseExec возвращает либо tx (если != nil), либо pool.
func (r *Repository) chooseExec(tx pgx.Tx) queryExecutor {
	if tx == nil {
		return r.pool
	}
	return tx
}
