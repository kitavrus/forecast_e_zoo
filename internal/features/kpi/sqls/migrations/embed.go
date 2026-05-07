// Package migrations содержит SQL-миграции kpi-фичи (Module 4).
//
// Используется dockertest-интеграционными тестами для применения
// схемы kpi.* поверх marts.* перед прогоном кейсов.
package migrations

import "embed"

// FS — embedded *.sql файлы миграций kpi (golang-migrate v4 совместимый формат).
//
//go:embed *.sql
var FS embed.FS
