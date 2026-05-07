// Package queries содержит embed.FS со всеми SQL-запросами source-adapter.
//
// Контракт Get(name): возвращает текст запроса по имени файла без расширения.
// Если файл не найден — паника на старте сервиса (defensive: непрошедший
// embed-файл означает рассинхрон контракта repository ↔ embed).
package queries

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.sql
var FS embed.FS

// Get возвращает SQL-запрос по логическому имени (имя файла без `.sql`).
// При отсутствии файла — panic (см. контракт пакета).
func Get(name string) string {
	s, err := getOrError(name)
	if err != nil {
		panic(err)
	}
	return s
}

// getOrError — internal-вариант для тестов.
func getOrError(name string) (string, error) {
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("queries.Get: bad name %q", name)
	}
	raw, err := FS.ReadFile(name + ".sql")
	if err != nil {
		return "", fmt.Errorf("queries.Get: %s.sql not found: %w", name, err)
	}
	return string(raw), nil
}

// MustGet — alias для Get (для читаемости в repository).
func MustGet(name string) string { return Get(name) }
