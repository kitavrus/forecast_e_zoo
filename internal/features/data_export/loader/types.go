package loader

import (
	"github.com/google/uuid"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
)

// EntityProgress — счётчики по одной сущности в рамках Load.
type EntityProgress struct {
	Entity      string
	LinesTotal  int64
	LinesFailed int64
	Inserted    int64
	Updated     int64
}

// LoadResult — итоговый отчёт.
type LoadResult struct {
	LoadID   uuid.UUID
	Status   models.LoadStatus
	Entities []EntityProgress
}

// Quality threshold (lines_failed/lines_total) — fail load если > этого значения.
const QualityThresholdRatio = 0.01 // 1%

// EntityOrder — фиксированный порядок сущностей для load-а.
// Сначала master (другие сущности на них ссылаются), потом facts.
var EntityOrder = []string{
	// master
	"category",
	"location",
	"supplier",
	"products",
	"product_barcodes",
	"store_assortment",
	"store_assortment_lifecycle_events",
	"supply_spec",
	"promo",
	"order_rule",
	"supply_plan",
	"master_change_log",
	// facts
	"receipt_line",
	"location_stock_snapshot",
	"stock_movement",
	"supplier_stock_snapshot",
}
