// Package config грузит настройки сервиса channel-router (Модуль 7) из переменных окружения.
package channelsappconfig

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config — корневой конфиг сервиса channel-router.
type Config struct {
	// HTTP
	HTTPAddr string `envconfig:"CHANNEL_ROUTING_HTTP_ADDR" default:":8087"`

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
	CronSchedule string `envconfig:"CHANNEL_ROUTING_CRON_SCHEDULE" default:"30 6 * * *"`
	CronTZ       string `envconfig:"CHANNEL_ROUTING_CRON_TZ" default:"Europe/Kyiv"`
	MaxPOs       int    `envconfig:"CHANNEL_ROUTING_MAX_POS" default:"500"`
	HTTPTimeout  int    `envconfig:"CHANNEL_ROUTING_HTTP_TIMEOUT_SEC" default:"30"`

	// Misc
	Env string `envconfig:"APP_ENV" default:"dev"`
}

// Load парсит переменные окружения в *Config.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("channels config: envconfig: %w", err)
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
