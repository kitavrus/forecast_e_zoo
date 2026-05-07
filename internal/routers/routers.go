// Package routers агрегирует feature-роутеры source-adapter binary.
//
// Только M1 (data_export). Остальные модули (M2..M7) — отдельные binary
// и регистрируют свои роуты внутри собственных *app/app.go.
package routers

import (
	"github.com/gofiber/fiber/v3"

	dataExportRouter "github.com/Kitavrus/e_zoo/internal/features/data_export/router"
)

// Register регистрирует data_export маршруты source-adapter.
func Register(app *fiber.App, dataExportDeps dataExportRouter.Deps) {
	dataExportRouter.Register(app, dataExportDeps)
}
