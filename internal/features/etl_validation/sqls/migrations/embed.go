// Package migrations содержит SQL-миграции golang-migrate/v4 для feature etl_validation,
// упакованные через go:embed для использования в репозитории и integration-тестах.
package migrations

import "embed"

// FS — embedded файловая система с *.sql файлами миграций фичи.
//
//go:embed *.sql
var FS embed.FS
