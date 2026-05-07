// Package routers агрегирует все feature-роутеры.
// Сейчас один data_export, в будущем — добавление новых features здесь.
package routers

import (
	"github.com/gofiber/fiber/v3"

	dataExportRouter "github.com/Kitavrus/e_zoo/internal/features/data_export/router"
)

// Register регистрирует все features в одной точке.
func Register(app *fiber.App, dataExportDeps dataExportRouter.Deps) {
	dataExportRouter.Register(app, dataExportDeps)
}
