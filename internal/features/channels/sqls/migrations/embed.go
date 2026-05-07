// Package migrations содержит SQL-миграции channel-routing фичи (Module 7).
//
// Используется dockertest-интеграционными тестами для применения схемы
// channels.* поверх orders.* (Module 6).
package migrations

import "embed"

// FS — embedded *.sql миграции (golang-migrate v4 совместимый формат).
//
//go:embed *.sql
var FS embed.FS
