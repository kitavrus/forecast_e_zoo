// Package migrations содержит SQL-миграции orders-фичи (Module 6).
//
// Используется dockertest-интеграционными тестами для применения
// схемы orders.* поверх forecast.* и marts.*.
package migrations

import "embed"

// FS — embedded *.sql файлы миграций orders (golang-migrate v4 совместимый формат).
//
//go:embed *.sql
var FS embed.FS
