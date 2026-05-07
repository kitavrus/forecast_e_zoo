// Package migrations содержит SQL-миграции golang-migrate/v4,
// упакованные через go:embed для использования в репозитории и в test/integration.
package migrations

import "embed"

// FS — embedded файловая система с *.sql файлами миграций.
//
//go:embed *.sql
var FS embed.FS
