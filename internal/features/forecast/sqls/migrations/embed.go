// Package migrations содержит SQL-миграции forecast-фичи (Module 5).
//
// Используется dockertest-интеграционными тестами для применения
// схемы forecast.* поверх marts.* перед прогоном кейсов.
package migrations

import "embed"

// FS — embedded *.sql файлы миграций forecast (golang-migrate v4 совместимый формат).
//
//go:embed *.sql
var FS embed.FS
