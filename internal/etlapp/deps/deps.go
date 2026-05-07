// Package deps собирает корневые зависимости сервиса etl (pgxpool, http.Client,
// jwt-signer). Реальная реализация интеграций — в последующих фазах (10/15).
package etldeps

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	etlconfig "github.com/Kitavrus/e_zoo/internal/etlapp/config"
)

// Deps — корневой набор зависимостей etl.
type Deps struct {
	Pool       *pgxpool.Pool
	HTTPClient *http.Client
	Logger     *slog.Logger
	Config     *etlconfig.Config
}

// BuildDeps инициализирует pgxpool + http.Client.
//
// Реальный JWT-signer и SourceClient собираются в фазах 10/15;
// здесь — только baseline.
func BuildDeps(ctx context.Context, cfg *etlconfig.Config, log *slog.Logger) (*Deps, error) {
	if cfg == nil {
		return nil, fmt.Errorf("etl deps: cfg is nil")
	}
	if log == nil {
		return nil, fmt.Errorf("etl deps: logger is nil")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("etl deps: pgxpool parse: %w", err)
	}
	if cfg.DBMaxConns > 0 {
		poolCfg.MaxConns = cfg.DBMaxConns
	}
	if cfg.DBMinConns > 0 {
		poolCfg.MinConns = cfg.DBMinConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("etl deps: pgxpool new: %w", err)
	}

	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
	}

	return &Deps{
		Pool:       pool,
		HTTPClient: httpClient,
		Logger:     log,
		Config:     cfg,
	}, nil
}

// Close корректно закрывает pgxpool.
func (d *Deps) Close() {
	if d == nil {
		return
	}
	if d.Pool != nil {
		d.Pool.Close()
	}
}
