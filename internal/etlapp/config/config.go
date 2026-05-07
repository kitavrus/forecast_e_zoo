// Package config грузит настройки сервиса etl (Модуль 2) из переменных окружения.
//
// Все поля помечены ENV-префиксом ETL_*. Для локального запуска без .env
// предусмотрены разумные default-значения.
package etlconfig

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config — корневой конфиг сервиса etl.
type Config struct {
	// Postgres
	DSN        string `envconfig:"ETL_DSN" required:"true"`
	DBMaxConns int32  `envconfig:"ETL_DB_MAX_CONNS" default:"20"`
	DBMinConns int32  `envconfig:"ETL_DB_MIN_CONNS" default:"2"`

	// HTTP
	HTTPPort int `envconfig:"ETL_HTTP_PORT" default:"8081"`

	// Source adapter (Модуль 1)
	SourceAdapterURL string        `envconfig:"ETL_SOURCE_ADAPTER_URL" default:"http://source-adapter:8080"`
	HTTPTimeout      time.Duration `envconfig:"ETL_HTTP_TIMEOUT" default:"30s"`
	HTTPRetryMax     int           `envconfig:"ETL_HTTP_RETRY_MAX" default:"3"`
	RetryBackoffCap  time.Duration `envconfig:"ETL_RETRY_BACKOFF_CAP" default:"30s"`

	// JWT исходящий (для запросов в source-adapter; роль x-flow-etl).
	JWTSigningKey string `envconfig:"ETL_JWT_SIGNING_KEY" default:""`
	JWTRole       string `envconfig:"ETL_JWT_ROLE" default:"x-flow-etl"`

	// JWT входящий (admin /admin/* — admin-cli/it-read; ADR-022).
	// Alg: HS256 (Secret) | RS256 (PublicKeyPath).
	AdminJWTAlg           string `envconfig:"ETL_ADMIN_JWT_ALG" default:"HS256"`
	AdminJWTSecret        string `envconfig:"ETL_ADMIN_JWT_SECRET" default:""`
	AdminJWTPublicKeyPath string `envconfig:"ETL_ADMIN_JWT_PUBLIC_KEY_PATH" default:""`

	// Scheduler
	CronSchedule string `envconfig:"ETL_CRON_SCHEDULE" default:"30 2 * * *"`
	CronTimezone string `envconfig:"ETL_CRON_TIMEZONE" default:"Europe/Kyiv"`

	// Validation
	ValidationRulesPath string  `envconfig:"ETL_VALIDATION_RULES_PATH" default:"./configs/etl_validation_rules.yaml"`
	QualityThreshold    float64 `envconfig:"ETL_QUALITY_THRESHOLD" default:"0.01"`

	// OTEL
	OTELExporterEndpoint string  `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:""`
	OTELTracesSamplerArg float64 `envconfig:"OTEL_TRACES_SAMPLER_ARG" default:"1.0"`

	// Misc
	LogLevel string `envconfig:"ETL_LOG_LEVEL" default:"info"`
	Env      string `envconfig:"ETL_ENV" default:"development"`
}

// Load парсит переменные окружения в *Config.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("etl config: envconfig: %w", err)
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
