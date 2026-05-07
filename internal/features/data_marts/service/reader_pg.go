package service

import (
	"context"
	"errors"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/data_marts/constants"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_marts/repository"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// PGReader — реализация MartReader поверх pgx-repository.
type PGReader struct {
	repo  *repository.Repository
	cache *versionCache
}

// NewPGReader создаёт reader с кэшем версий 60s TTL (ADR-004).
func NewPGReader(repo *repository.Repository) *PGReader {
	return &PGReader{
		repo:  repo,
		cache: newVersionCache(time.Duration(constants.CacheTTLSeconds) * time.Second),
	}
}

// List — все mart'ы + их текущие версии.
//
// Если committed run отсутствует, repo возвращает 5 строк с nil EtlRunID/CommittedAt.
// Преобразуем в models.MartInfo (zero value для unfilled marts).
func (p *PGReader) List(ctx context.Context) ([]models.MartInfo, error) {
	versions, err := p.repo.ListMartVersions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.MartInfo, 0, len(versions))
	for _, v := range versions {
		info := models.MartInfo{Name: v.Name}
		if v.EtlRunID != nil {
			info.EtlRunID = *v.EtlRunID
		}
		if v.CommittedAt != nil {
			info.CommittedAt = *v.CommittedAt
		}
		out = append(out, info)
	}
	return out, nil
}

// GetVersion возвращает текущую версию mart'а (через cache).
//
// MVP: версия одна на все mart'ы (один full-run наполняет все 5).
// Cache key = mart_name (на случай будущего mart_refresh, который обновит один mart).
func (p *PGReader) GetVersion(ctx context.Context, name string) (models.MartVersion, error) {
	if !constants.IsKnownMart(name) {
		return models.MartVersion{}, errorspkg.ErrNotFound.WithMessage("mart not found: " + name)
	}
	if v, ok := p.cache.get(name); ok {
		return v, nil
	}
	row, err := p.repo.GetCurrentVersion(ctx)
	if err != nil {
		return models.MartVersion{}, err
	}
	v := models.MartVersion{
		Name:        name,
		EtlRunID:    row.EtlRunID,
		CommittedAt: row.CommittedAt,
	}
	p.cache.put(name, v)
	return v, nil
}

// GetSchema возвращает schema mart'а из hardcoded таблицы (ADR-002).
func (p *PGReader) GetSchema(_ context.Context, name string) (models.MartSchema, error) {
	fields, ok := schemas[name]
	if !ok {
		return models.MartSchema{}, errorspkg.ErrNotFound.WithMessage("mart not found: " + name)
	}
	return models.MartSchema{Name: name, Fields: fields}, nil
}

// Read — стримит страницу строк mart'а.
// cursorEnc — opaque base64; пустая строка = начало стрима.
func (p *PGReader) Read(
	ctx context.Context, name, cursorEnc string, limit int,
) ([]models.MartRow, string, models.MartVersion, error) {
	if !constants.IsKnownMart(name) {
		return nil, "", models.MartVersion{}, errorspkg.ErrNotFound.WithMessage("mart not found: " + name)
	}

	var cur models.Cursor
	if err := cur.Decode(cursorEnc); err != nil {
		return nil, "", models.MartVersion{}, errorspkg.ErrBadRequest.WithMessage("invalid cursor")
	}

	// Если в cursor явно указан run — используем его (cross-page consistency).
	// Иначе — берём текущую committed-версию через GetVersion (cached).
	var version models.MartVersion
	switch {
	case cur.EtlRunID.String() != "00000000-0000-0000-0000-000000000000":
		// Cursor задаёт run явно. Получим committed_at для ответа (best-effort).
		v, err := p.GetVersion(ctx, name)
		if err != nil {
			return nil, "", models.MartVersion{}, err
		}
		// Override: используем run из cursor; committed_at — текущий (информационно).
		v.EtlRunID = cur.EtlRunID
		version = v
	default:
		v, err := p.GetVersion(ctx, name)
		if err != nil {
			// Сервис недоступен (нет committed run'а вообще) — пробрасываем как есть.
			return nil, "", models.MartVersion{}, err
		}
		version = v
		cur.EtlRunID = v.EtlRunID
	}

	rows, nextPK, err := p.repo.SelectMartRows(ctx, name, version.EtlRunID, cur, limit)
	if err != nil {
		// Если sentinel ErrServiceUnavailable / ErrNotFound — не сбрасываем cache.
		if !errors.Is(err, errorspkg.ErrServiceUnavailable) && !errors.Is(err, errorspkg.ErrNotFound) {
			p.cache.invalidate(name)
		}
		return nil, "", models.MartVersion{}, err
	}

	// nextCursor: если nextPK пуст — стрим закончен.
	if nextPK == "" {
		return rows, "", version, nil
	}
	nextCursor := models.Cursor{EtlRunID: version.EtlRunID, LastPK: nextPK}
	enc, encErr := nextCursor.Encode()
	if encErr != nil {
		return nil, "", models.MartVersion{}, errorspkg.ErrInternal.Wrap(encErr)
	}
	return rows, enc, version, nil
}
