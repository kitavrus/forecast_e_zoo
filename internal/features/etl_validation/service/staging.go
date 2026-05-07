// Package service: stage-mapping для full-run pipeline.
//
// Здесь живёт описание соответствия entity (имя в API source-adapter и YAML
// validation-rules) ↔ staging-таблица + порядок колонок для CopyFrom.
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/constants"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/extractor"
	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
)

// stagingSpec — описание staging-таблицы для одной entity.
type stagingSpec struct {
	table   string   // имя TEMP-таблицы (без схемы; CopyFrom использует pg_temp по search_path)
	columns []string // упорядоченный список колонок для CopyFrom
}

// stagingByEntity — соответствие entity → staging-спецификация.
//
// Имена entity берутся из constants.AllowedEntities + constants.AllowedEntities
// и должны совпадать с теми, что использует validation YAML и API source-adapter
// (single-source-of-truth — constants).
//
//nolint:gochecknoglobals // справочная константа.
var stagingByEntity = map[string]stagingSpec{
	constants.EntityReceiptLine: {
		table: "stg_receipt_line",
		columns: []string{
			"receipt_id", "location_id", "product_id", "line_kind",
			"qty", "unit_price_list", "unit_price_paid", "discount_amount",
			"had_promo", "promo_type", "event_time",
		},
	},
	constants.EntityStockOnHand: {
		table:   "stg_stock_on_hand",
		columns: []string{"product_id", "location_id", "qty_on_hand", "qty_in_transit", "as_of_date"},
	},
	constants.EntityProduct: {
		table:   "stg_products",
		columns: []string{"id", "name", "category_id", "unit_id", "is_active"},
	},
	constants.EntityLocation: {
		table:   "stg_locations",
		columns: []string{"id", "name", "is_active"},
	},
	constants.EntitySupplier: {
		table:   "stg_suppliers",
		columns: []string{"id", "name", "is_active"},
	},
	constants.EntityOrderRule: {
		table: "stg_order_rule",
		columns: []string{
			"id", "product_id", "location_id", "formula",
			"safety_stock", "forecast_horizon_days", "daily_demand",
			"min_qty", "max_qty", "supplier_id", "lead_time_days", "is_active",
		},
	},
	constants.EntitySupplySpec: {
		table: "stg_supply_spec",
		columns: []string{
			"id", "supplier_id", "product_id", "location_id",
			"lead_time_days", "safety_stock", "min_qty", "max_qty", "is_active",
		},
	},
	constants.EntityReceivingDetail: {
		table: "stg_receiving_details",
		columns: []string{
			"supplier_id", "product_id", "delivery_date",
			"fill_rate", "otif", "lead_time_actual",
			"qty_short", "qty_damaged", "qty_returned", "late",
		},
	},
	constants.EntityPromo: {
		table:   "stg_promo",
		columns: []string{"id", "product_id", "type", "date_from", "date_to"},
	},
	constants.EntityStoreAssortment: {
		table:   "stg_store_assortment",
		columns: []string{"product_id", "location_id", "valid_from", "valid_to", "lifecycle_state"},
	},
}

// extractAndStageResult — итог extract+staging stage.
type extractAndStageResult struct {
	dataset    *validation.Dataset
	rowsByEnt  map[string][]validation.Row
	linesTotal int64
}

// extractAllEntities стримит NDJSON для каждого entity из constants.AllowedEntities,
// декодирует в validation.Row (map[string]any), наполняет Dataset и возвращает rows
// для последующей COPY-загрузки внутри loader-tx.
//
// Контракт ошибок: возвращает (nil, error) на любой проблеме сети/парсинга;
// caller (pipeline.runAsync) переводит run в failed с reason.
func extractAllEntities(ctx context.Context, extr Extractor, snap extractor.Snapshot) (*extractAndStageResult, error) {
	ds := validation.NewDataset()
	rowsByEnt := make(map[string][]validation.Row, len(constants.AllowedEntities))
	var linesTotal int64

	for _, entity := range constants.AllowedEntities {
		rows, err := streamEntityRows(ctx, extr, entity, snap)
		if err != nil {
			return nil, fmt.Errorf("extract %s: %w", entity, err)
		}
		ds.SetEntity(entity, rows)
		rowsByEnt[entity] = rows
		linesTotal += int64(len(rows))
	}

	return &extractAndStageResult{
		dataset:    ds,
		rowsByEnt:  rowsByEnt,
		linesTotal: linesTotal,
	}, nil
}

// streamEntityRows скачивает NDJSON по одной entity, декодируя каждую строку в Row.
func streamEntityRows(ctx context.Context, extr Extractor, entity string, snap extractor.Snapshot) ([]validation.Row, error) {
	rd, err := extr.StreamEntity(ctx, entity, snap.LoadID, "")
	if err != nil {
		return nil, fmt.Errorf("stream: %w", err)
	}
	defer func() { _ = rd.Close() }()

	rows := make([]validation.Row, 0, 64)
	for {
		var row validation.Row
		err := rd.Next(&row)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// populateStaging возвращает callback для loader.ApplyParams.PopulateStaging,
// который выполняет COPY rowsByEnt[entity] → pg_temp.stg_<table> для каждой
// зарегистрированной entity. Если для entity нет staging-спеки — пропускается.
//
// Каждый COPY выполняется в той же tx, что и mart-builder-ы — обеспечивая
// атомарность Q-008 (snapshot-flip).
func populateStaging(rowsByEnt map[string][]validation.Row) func(ctx context.Context, tx pgx.Tx) error {
	return func(ctx context.Context, tx pgx.Tx) error {
		for entity, rows := range rowsByEnt {
			spec, ok := stagingByEntity[entity]
			if !ok {
				// Entity без staging-спеки — пропускаем (например, если YAML добавил
				// новую entity до миграции 1001). Не ошибка, но логировать стоит.
				continue
			}
			if len(rows) == 0 {
				continue
			}
			src := newRowSource(spec.columns, rows)
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{spec.table}, spec.columns, src); err != nil {
				return fmt.Errorf("copy %s: %w", spec.table, err)
			}
		}
		return nil
	}
}

// rowSource — pgx.CopyFromSource поверх []validation.Row.
//
// Преобразует значения map → []any в порядке, заданном columns. Поддерживает
// базовые типы, которые ожидают staging-таблицы (TEXT/NUMERIC/INTEGER/BOOLEAN/DATE/TIMESTAMPTZ).
type rowSource struct {
	columns []string
	rows    []validation.Row
	idx     int
}

func newRowSource(columns []string, rows []validation.Row) *rowSource {
	return &rowSource{columns: columns, rows: rows, idx: -1}
}

// Next implements pgx.CopyFromSource.
func (s *rowSource) Next() bool {
	s.idx++
	return s.idx < len(s.rows)
}

// Values implements pgx.CopyFromSource.
func (s *rowSource) Values() ([]any, error) {
	if s.idx >= len(s.rows) {
		return nil, errors.New("rowSource: out of range")
	}
	row := s.rows[s.idx]
	out := make([]any, len(s.columns))
	for i, col := range s.columns {
		v, ok := row[col]
		if !ok || v == nil {
			out[i] = nil
			continue
		}
		conv, err := convertValue(col, v)
		if err != nil {
			return nil, fmt.Errorf("col %s: %w", col, err)
		}
		out[i] = conv
	}
	return out, nil
}

// Err implements pgx.CopyFromSource.
func (s *rowSource) Err() error { return nil }

// convertValue приводит значение из decoded JSON (string, float64, bool, nil)
// к типу, который ожидает соответствующая staging-колонка.
//
// JSON-decoder json.Unmarshal в map[string]any выдаёт:
//   - все числа → float64;
//   - даты/времена → string (ISO8601);
//   - boolean → bool.
//
// Для DATE/TIMESTAMPTZ-колонок возвращаем time.Time, для остальных — оригинальное
// значение (pgx сам приведёт float64→NUMERIC, string→TEXT).
func convertValue(col string, v any) (any, error) {
	if isDateColumn(col) {
		return parseDate(v)
	}
	if isTimestampColumn(col) {
		return parseTimestamp(v)
	}
	// number-type columns — pgx сам приведёт float64/int к NUMERIC/INTEGER.
	// boolean / text — pgx тоже сам справится.
	return v, nil
}

func isDateColumn(col string) bool {
	switch col {
	case "as_of_date", "delivery_date", "date_from", "date_to", "valid_from", "valid_to":
		return true
	}
	return false
}

func isTimestampColumn(col string) bool {
	return col == "event_time"
}

// parseDate принимает string ("YYYY-MM-DD" или RFC3339) или time.Time.
func parseDate(v any) (any, error) {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil, nil //nolint:nilnil // empty string → SQL NULL
		}
		// Try YYYY-MM-DD first (короткий формат от source-adapter).
		if t, err := time.Parse("2006-01-02", x); err == nil {
			return t, nil
		}
		// Fallback на RFC3339.
		t, err := time.Parse(time.RFC3339, x)
		if err != nil {
			return nil, fmt.Errorf("parse date %q: %w", x, err)
		}
		return t, nil
	case time.Time:
		return x, nil
	default:
		return nil, fmt.Errorf("unexpected type %T for date column", v)
	}
}

// parseTimestamp принимает RFC3339 string или time.Time.
func parseTimestamp(v any) (any, error) {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil, nil //nolint:nilnil // empty string → SQL NULL
		}
		t, err := time.Parse(time.RFC3339, x)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", x, err)
		}
		return t, nil
	case time.Time:
		return x, nil
	case float64:
		// Unix seconds — на всякий случай.
		return time.Unix(int64(x), 0).UTC(), nil
	default:
		return nil, fmt.Errorf("unexpected type %T for timestamp column", v)
	}
}

