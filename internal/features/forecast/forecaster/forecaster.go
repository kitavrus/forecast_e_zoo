// Package forecaster — прогнозирование спроса (Module 5).
//
// Контракт Forecaster — pluggable: текущая impl MovingAverageForecaster
// использует SMA + day-of-week + week-of-year multipliers. Будущие impls
// (Prophet, LightGBM, нейросети) могут заменять реализацию без изменения
// engine/orchestration.
package forecaster

import (
	"context"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/forecast/models"
)

// Forecaster — интерфейс прогнозировщика.
//
// PredictBatch принимает:
//   - history: точки спроса (отсортированные по date) в окне lookback.
//   - asOf: дата, ОТ которой начинается горизонт (forecast_date = asOf, asOf+1, ...).
//   - horizonDays: количество дней прогноза вперёд.
//
// Возвращает []Forecast (без RunID — engine проставляет потом).
//
// Группировка по (product_id, location_id) — обязанность реализации.
type Forecaster interface {
	PredictBatch(
		ctx context.Context,
		history []models.DemandPoint,
		asOf time.Time,
		horizonDays int,
	) ([]models.Forecast, error)
}

// ModelName — имя реализации forecaster (для записи в forecasts.model_name).
type ModelName interface {
	ModelName() string
}
