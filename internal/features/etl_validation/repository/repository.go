// Package repository — pgx/v5 + pgxpool слой доступа к БД для feature etl_validation.
//
// Контракты:
//   - все методы (ctx, ...) (..., error);
//   - sentinel-ошибки маппятся через pkg/errorspkg
//     (pgx.ErrNoRows → ErrEtlRunNotFound для GetByID;
//      pgconn 23505 → ErrAlreadyExists);
//   - SQL-запросы — go:embed из internal/features/etl_validation/sqls/queries.
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

// queryExecutor — единый интерфейс «исполнителя запроса», абстрагирует tx и pool.
type queryExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// chooseExec возвращает либо tx (если != nil), либо pool.
func (r *Repository) chooseExec(tx pgx.Tx) queryExecutor {
	if tx != nil {
		return tx
	}
	return r.pool
}

// mapPgError конвертирует pgconn.PgError в sentinel из errorspkg.
// pgx.ErrNoRows маппится индивидуально каждым методом
// (для разных операций — разные NotFound sentinels).
func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return errorspkg.ErrAlreadyExists.Wrap(err)
	}
	return err
}
