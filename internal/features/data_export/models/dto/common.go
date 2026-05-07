// Package dto содержит API-DTO (request/response) source-adapter.
// Поля DTO — это shape, отдаваемый/принимаемый через HTTP, и могут совпадать
// с domain-models. Связь — через mappers/.
package dto

// PageRequest — query-параметры пагинации (GET /v1/*).
type PageRequest struct {
	Cursor string `query:"cursor" validate:"omitempty,max=1024"`
	Limit  int    `query:"limit" validate:"min=1,max=10000"`
}

// PageResponse — обёртка одной страницы для NDJSON/JSON-response.
type PageResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// IfNoneMatchHeader — имя заголовка ETag conditional GET.
const IfNoneMatchHeader = "If-None-Match"

// ETagHeader — имя заголовка ответа ETag.
const ETagHeader = "ETag"

// LimitDefault — лимит по умолчанию, если не задан.
const LimitDefault = 1000

// LimitMax — верхняя граница (см. validate tag).
const LimitMax = 10000
