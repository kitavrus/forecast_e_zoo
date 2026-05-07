// Package logger создаёт корневой slog-логгер.
//
// Используем JSON-handler как единый формат для prod/dev,
// чтобы любые утилиты (vector/loki/grafana) могли парсить логи без regex.
package logger

import (
	"log/slog"
	"os"
)

// New создаёт slog.Logger с JSON-handler'ом и указанным уровнем.
func New(level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})
	return slog.New(handler)
}
