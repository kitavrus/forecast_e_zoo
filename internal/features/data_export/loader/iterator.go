// Package loader содержит интерфейс SourceReader и его in-memory реализацию
// erp_e_zoo_reader (MVP-stub). Реальная REST/SOAP/SFTP интеграция блокируется
// открытыми вопросами Q-001..Q-003 (контракт + auth) и реализуется отдельно.
package loader

import "context"

// PageIterator — generic-итератор страниц/строк одной сущности.
// Контракт:
//   - Next возвращает true, пока есть строки. После false — Item() undefined.
//   - Err() возвращает ошибку, если итерация прервалась.
//   - Close — освобождает ресурсы (для REST/SOAP важно).
type PageIterator[T any] interface {
	Next(ctx context.Context) bool
	Item() T
	Err() error
	Close() error
}

// sliceIterator — простая in-memory реализация PageIterator над slice'ом.
type sliceIterator[T any] struct {
	items  []T
	idx    int
	cur    T
	closed bool
	err    error
}

// newSliceIterator создаёт итератор поверх slice'а (snapshot of items).
func newSliceIterator[T any](items []T) *sliceIterator[T] {
	cp := make([]T, len(items))
	copy(cp, items)
	return &sliceIterator[T]{items: cp, idx: -1}
}

// Next продвигает итератор. После Close — false, не двигаясь.
func (s *sliceIterator[T]) Next(_ context.Context) bool {
	if s.closed {
		return false
	}
	s.idx++
	if s.idx >= len(s.items) {
		var zero T
		s.cur = zero
		return false
	}
	s.cur = s.items[s.idx]
	return true
}

// Item возвращает последнюю добытую через Next() строку.
func (s *sliceIterator[T]) Item() T { return s.cur }

// Err — пока всегда nil для in-memory; место под будущие источники.
func (s *sliceIterator[T]) Err() error { return s.err }

// Close помечает итератор закрытым.
func (s *sliceIterator[T]) Close() error {
	s.closed = true
	return nil
}
