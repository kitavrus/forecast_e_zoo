// Package config грузит настройки сервиса order-builder (Модуль 6) из переменных окружения.
package ordersappconfig

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config — корневой конфиг сервиса order-builder.
type Config struct {
	// HTTP
	HTTPAddr string `envconfig:"ORDER_BUILDER_HTTP_ADDR" default:":8086"`

	// Postgres
	DBDsn      string `envconfig:"DB_DSN" required:"true"`
	DBMaxConns int32  `envconfig:"DB_MAX_CONNS" default:"20"`
	DBMinConns int32  `envconfig:"DB_MIN_CONNS" default:"2"`

	// Logging
	LogLevel string `envconfig:"LOG_LEVEL" default:"INFO"`

	// JWT (входящий)
	JWTAlg            string `envconfig:"JWT_ALG" default:"HS256"`
	JWTSecret         string `envconfig:"JWT_SECRET" default:""`
	JWTPublicKeyPath  string `envconfig:"JWT_PUBLIC_KEY_PATH" default:""`

	// Scheduler
	CronSchedule string `envconfig:"ORDER_BUILDER_CRON_SCHEDULE" default:"0 6 * * *"`
	CronTZ       string `envconfig:"ORDER_BUILDER_CRON_TZ" default:"Europe/Kyiv"`
	MaxPlans     int    `envconfig:"ORDER_BUILDER_MAX_PLANS" default:"500"`

	// Misc
	Env string `envconfig:"APP_ENV" default:"dev"`
}

// Load парсит переменные окружения в *Config.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("orders config: envconfig: %w", err)
	}
	return &cfg, nil
}

// SlogLevel конвертирует строковый уровень логирования в slog.Level.
func (c *Config) SlogLevel() slog.Level {
	switch strings.ToUpper(strings.TrimSpace(c.LogLevel)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
