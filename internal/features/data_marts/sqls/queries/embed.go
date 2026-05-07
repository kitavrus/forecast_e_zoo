// Package queries содержит embed.FS со всеми SQL-запросами фичи data_marts.
//
// Контракт MustGet(name): возвращает текст запроса по имени файла без расширения.
// Если файл не найден — паника на старте сервиса (defensive: непрошедший
// embed-файл означает рассинхрон контракта repository ↔ embed).
package queries

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.sql
var fs embed.FS

// MustGet возвращает SQL по логическому имени или паникует.
func MustGet(name string) string {
	s, err := getOrError(name)
	if err != nil {
		panic(err)
	}
	return s
}

func getOrError(name string) (string, error) {
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("data_marts queries.Get: bad name %q", name)
	}
	raw, err := fs.ReadFile(name + ".sql")
	if err != nil {
		return "", fmt.Errorf("data_marts queries.Get: %s.sql not found: %w", name, err)
	}
	return string(raw), nil
}
