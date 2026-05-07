// Package routers агрегирует все feature-роутеры.
package routers

import (
	"github.com/gofiber/fiber/v3"

	dataExportRouter "github.com/Kitavrus/e_zoo/internal/features/data_export/router"
	dataMartsRouter "github.com/Kitavrus/e_zoo/internal/features/data_marts/router"
	forecastRouter "github.com/Kitavrus/e_zoo/internal/features/forecast/router"
	kpiRouter "github.com/Kitavrus/e_zoo/internal/features/kpi/router"
	ordersRouter "github.com/Kitavrus/e_zoo/internal/features/orders/router"
)

// Register регистрирует все features в одной точке.
//
// data_marts, kpi, forecast, orders встраиваются в source-adapter binary как slim features
// (читают/пишут общую БД). Если *Deps.Handler == nil — регистрация
// соответствующего фича-роутера пропускается.
func Register(
	app *fiber.App,
	dataExportDeps dataExportRouter.Deps,
	dataMartsDeps dataMartsRouter.Deps,
	kpiDeps kpiRouter.Deps,
	forecastDeps forecastRouter.Deps,
	ordersDeps ordersRouter.Deps,
) {
	dataExportRouter.Register(app, dataExportDeps)
	dataMartsRouter.Register(app, dataMartsDeps)
	kpiRouter.Register(app, kpiDeps)
	forecastRouter.Register(app, forecastDeps)
	ordersRouter.Register(app, ordersDeps)
}
