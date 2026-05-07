// Package config грузит настройки сервиса из переменных окружения.
//
// Используется envconfig (kelseyhightower) — все поля имеют default-теги,
// чтобы локальный запуск без .env поднимался с разумными значениями.
package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config — корневой конфиг сервиса source-adapter.
//
// Имена ENV соответствуют design-infrastructure.md §3.
type Config struct {
	// HTTP
	HTTPAddr string `envconfig:"HTTP_ADDR" default:":8080"`

	// Postgres
	DBDsn      string `envconfig:"DB_DSN" required:"true"`
	DBMaxConns int32  `envconfig:"DB_MAX_CONNS" default:"20"`
	DBMinConns int32  `envconfig:"DB_MIN_CONNS" default:"2"`

	// Logging
	LogLevel string `envconfig:"LOG_LEVEL" default:"INFO"`

	// JWT
	JWTAlg            string   `envconfig:"JWT_ALG" default:"HS256"`
	JWTSecret         string   `envconfig:"JWT_SECRET" default:""`
	JWTPublicKeyPath  string   `envconfig:"JWT_PUBLIC_KEY_PATH" default:""`
	JWTAdminRole      string   `envconfig:"JWT_ADMIN_ROLE" default:"admin-cli"`
	JWTReadRolesRaw   string   `envconfig:"JWT_READ_ROLES" default:"x-flow-etl,it-read"`
	JWTReadRoles      []string `ignored:"true"`

	// Scheduler
	SourceAdapterCron string `envconfig:"SOURCE_ADAPTER_CRON_SCHEDULE" default:"0 2 * * *"`
	SourceAdapterTZ   string `envconfig:"SOURCE_ADAPTER_TZ" default:"Europe/Kyiv"`

	// KPI Engine (Module 4)
	KPICronSchedule string `envconfig:"KPI_CRON_SCHEDULE" default:"0 4 * * *"`
	KPICronTZ       string `envconfig:"KPI_CRON_TZ" default:"Europe/Kyiv"`

	// Forecast Engine (Module 5)
	ForecastCronSchedule string `envconfig:"FORECAST_CRON_SCHEDULE" default:"0 5 * * *"`
	ForecastCronTZ       string `envconfig:"FORECAST_CRON_TZ" default:"Europe/Kyiv"`
	ForecastHorizonDays  int    `envconfig:"FORECAST_HORIZON_DAYS" default:"14"`

	// Quality
	QualityThresholdPct float64 `envconfig:"QUALITY_THRESHOLD_PCT" default:"1.0"`

	// ERP backend
	ERPBaseURL          string        `envconfig:"ERP_BASE_URL" default:""`
	ERPAuthMode         string        `envconfig:"ERP_AUTH_MODE" default:"none"`
	ERPAPIKey           string        `envconfig:"ERP_API_KEY" default:""`
	ERPOAuthTokenURL    string        `envconfig:"ERP_OAUTH_TOKEN_URL" default:""`
	ERPOAuthClientID    string        `envconfig:"ERP_OAUTH_CLIENT_ID" default:""`
	ERPOAuthSecret      string        `envconfig:"ERP_OAUTH_CLIENT_SECRET" default:""`
	ERPMTLSCertPath     string        `envconfig:"ERP_MTLS_CERT_PATH" default:""`
	ERPMTLSKeyPath      string        `envconfig:"ERP_MTLS_KEY_PATH" default:""`
	ERPHTTPTimeout      time.Duration `envconfig:"ERP_HTTP_TIMEOUT" default:"30s"`
	ERPRetryMax         int           `envconfig:"ERP_RETRY_MAX" default:"3"`
	ERPRetryBackoffCap  time.Duration `envconfig:"ERP_RETRY_BACKOFF_CAP" default:"30s"`

	// Exports storage
	ExportsBaseDir     string        `envconfig:"EXPORTS_BASE_DIR" default:"/var/exports"`
	ExportsRetention   time.Duration `envconfig:"EXPORTS_RETENTION" default:"24h"`
	ExportsInlineMaxMB int           `envconfig:"EXPORTS_INLINE_MAX_MB" default:"50"`

	// Audit / reject log retention
	AuditRetention      time.Duration `envconfig:"AUDIT_RETENTION" default:"2160h"`
	RejectLogRetention  time.Duration `envconfig:"REJECT_LOG_RETENTION" default:"2160h"`

	// YAML rules
	ValidationRulesPath     string `envconfig:"VALIDATION_RULES_PATH" default:"/etc/source-adapter/validation_rules.yaml"`
	MasterTrackedFieldsPath string `envconfig:"MASTER_TRACKED_FIELDS_PATH" default:"/etc/source-adapter/master_tracked_fields.yaml"`

	// Stale load recovery (Q-015 / ADR-015)
	StaleLoadTimeout time.Duration `envconfig:"SOURCE_ADAPTER_STALE_LOAD_TIMEOUT" default:"1h"`

	// Misc
	AppEnv         string `envconfig:"APP_ENV" default:"dev"`
	PrometheusPath string `envconfig:"PROMETHEUS_PATH" default:"/metrics"`
}

// Load читает env и возвращает заполненный Config или ошибку валидации.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("envconfig: %w", err)
	}

	// Парсим список ролей на чтение (CSV).
	if cfg.JWTReadRolesRaw != "" {
		parts := strings.Split(cfg.JWTReadRolesRaw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		cfg.JWTReadRoles = out
	}

	return &cfg, nil
}

// SlogLevel конвертирует строковый уровень логирования в slog.Level.
// Неизвестные значения трактуются как INFO.
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
