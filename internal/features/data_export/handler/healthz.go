package handler

import (
	"context"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
)

// DBPinger — то, что должно поддерживать healthz.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// HealthzHandler — GET /healthz.
type HealthzHandler struct {
	pinger DBPinger
}

// NewHealthzHandler — конструктор.
func NewHealthzHandler(p DBPinger) *HealthzHandler { return &HealthzHandler{pinger: p} }

// Get — GET /healthz.
func (h *HealthzHandler) Get(c fiber.Ctx) error {
	resp := dto.HealthzResponse{
		Status:  "ok",
		DB:      "ok",
		Version: buildVersion(),
	}
	if err := h.pinger.Ping(c.Context()); err != nil {
		resp.Status = "degraded"
		resp.DB = "unreachable"
		return c.Status(fiber.StatusServiceUnavailable).JSON(resp)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// buildVersion — VCS info из binary metadata, если доступно.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			if len(s.Value) > 12 {
				return s.Value[:12]
			}
			return s.Value
		}
	}
	return "dev"
}
