// Package queries содержит embed.FS со SQL-запросами фичи orders.
//
// Контракт MustGet(name): возвращает текст запроса по имени файла без расширения.
// При отсутствии файла — паника на старте сервиса (защита от рассинхрона
// repository ↔ embed FS).
package queries

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.sql
var fs embed.FS

// MustGet возвращает SQL-запрос по логическому имени или паникует.
func MustGet(name string) string {
	s, err := getOrError(name)
	if err != nil {
		panic(err)
	}
	return s
}

func getOrError(name string) (string, error) {
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("orders queries.Get: bad name %q", name)
	}
	raw, err := fs.ReadFile(name + ".sql")
	if err != nil {
		return "", fmt.Errorf("orders queries.Get: %s.sql not found: %w", name, err)
	}
	return string(raw), nil
}
