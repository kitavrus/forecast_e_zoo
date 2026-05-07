// Package calibration — иерархический matcher калибровок.
//
// Иерархия (от specific к generic) для одного KPI:
//
//	product_location > location > supplier > category > global
//
// Resolver получает срез калибровок один раз (loadAll) и затем in-memory
// решает (kpi, scope-keys) → matched calibration. Один scope — один матч.
// Если ничего не нашлось — возвращается dummy "global" с пустыми params.
package calibration

import (
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/kpi/constants"
	"github.com/Kitavrus/e_zoo/internal/features/kpi/models"
)

// ScopeKeys — opportunity keys для матчинга калибровки.
//
// Resolver ищет наиболее specific scope, для которого есть калибровка.
// nil-поля пропускаются.
type ScopeKeys struct {
	ProductLocation *string // опциональный составной ключ "product_id|location_id"
	LocationID      *string
	SupplierID      *string
	CategoryID      *string
}

// Resolver — in-memory matcher. Не safe для конкурентного writes; чтение OK.
type Resolver struct {
	// byKpi[kpiName][scopeType][scopeID] = calibration
	byKpi map[string]map[string]map[string]models.KpiCalibration
}

// NewResolver строит индекс по списку всех калибровок.
func NewResolver(all []models.KpiCalibration) *Resolver {
	idx := make(map[string]map[string]map[string]models.KpiCalibration, 4) //nolint:mnd // 4 — кол-во KPI с запасом
	for _, c := range all {
		if _, ok := idx[c.KpiName]; !ok {
			idx[c.KpiName] = make(map[string]map[string]models.KpiCalibration, len(constants.ScopeTypes))
		}
		if _, ok := idx[c.KpiName][c.ScopeType]; !ok {
			idx[c.KpiName][c.ScopeType] = make(map[string]models.KpiCalibration, 8) //nolint:mnd
		}
		key := ""
		if c.ScopeID != nil {
			key = *c.ScopeID
		}
		idx[c.KpiName][c.ScopeType][key] = c
	}
	return &Resolver{byKpi: idx}
}

// Resolve возвращает наиболее specific калибровку для (kpiName, keys).
//
// Priority order (от specific к generic):
//  1. product_location  — keys.ProductLocation
//  2. location          — keys.LocationID
//  3. supplier          — keys.SupplierID
//  4. category          — keys.CategoryID
//  5. global            — fallback
//
// Если ни одной калибровки в БД нет (даже global) — возвращает dummy
// модель с zero ID и пустыми params (engine использует hardcoded defaults).
//
//nolint:cyclop // линейная цепочка по 5 фиксированным уровням иерархии — без условных скачков
func (r *Resolver) Resolve(kpiName string, keys ScopeKeys) models.KpiCalibration {
	scopes := r.byKpi[kpiName]
	if scopes == nil {
		return emptyCalibration(kpiName)
	}

	if keys.ProductLocation != nil {
		if c, ok := scopes[constants.ScopeTypeProductLocation][*keys.ProductLocation]; ok {
			return c
		}
	}
	if keys.LocationID != nil {
		if c, ok := scopes[constants.ScopeTypeLocation][*keys.LocationID]; ok {
			return c
		}
	}
	if keys.SupplierID != nil {
		if c, ok := scopes[constants.ScopeTypeSupplier][*keys.SupplierID]; ok {
			return c
		}
	}
	if keys.CategoryID != nil {
		if c, ok := scopes[constants.ScopeTypeCategory][*keys.CategoryID]; ok {
			return c
		}
	}
	if c, ok := scopes[constants.ScopeTypeGlobal][""]; ok {
		return c
	}
	return emptyCalibration(kpiName)
}

// emptyCalibration — dummy для случая "ни одна калибровка для KPI не задана".
// ID = uuid.Nil сигнализирует engine использовать hardcoded defaults.
func emptyCalibration(kpiName string) models.KpiCalibration {
	return models.KpiCalibration{
		ID:        uuid.Nil,
		KpiName:   kpiName,
		ScopeType: constants.ScopeTypeGlobal,
		Params:    []byte(`{}`),
	}
}
