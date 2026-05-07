// Package routers агрегирует все feature-роутеры.
package routers

import (
	"github.com/gofiber/fiber/v3"

	dataExportRouter "github.com/Kitavrus/e_zoo/internal/features/data_export/router"
	dataMartsRouter "github.com/Kitavrus/e_zoo/internal/features/data_marts/router"
)

// Register регистрирует все features в одной точке.
//
// data_marts встраивается в source-adapter binary как slim feature
// (read-only API над marts.*) — не отдельный binary.
func Register(
	app *fiber.App,
	dataExportDeps dataExportRouter.Deps,
	dataMartsDeps dataMartsRouter.Deps,
) {
	dataExportRouter.Register(app, dataExportDeps)
	dataMartsRouter.Register(app, dataMartsDeps)
}
