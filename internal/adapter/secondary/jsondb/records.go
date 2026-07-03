package jsondb

// Generic-операции над записями коллекции. Вынесены в движок, чтобы фичевые
// адаптеры не повторяли одинаковые циклы find/replace/delete на каждую
// сущность.

// Upsert заменяет первую запись, совпавшую по match, или добавляет item в конец.
func Upsert[T any](items []T, match func(T) bool, item T) []T {
	for i := range items {
		if match(items[i]) {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

// Find возвращает первую запись, совпавшую по match.
func Find[T any](items []T, match func(T) bool) (T, bool) {
	for _, it := range items {
		if match(it) {
			return it, true
		}
	}
	var zero T
	return zero, false
}

// Remove удаляет первую запись, совпавшую по match. Второй результат — нашлась ли.
func Remove[T any](items []T, match func(T) bool) ([]T, bool) {
	for i := range items {
		if match(items[i]) {
			return append(items[:i], items[i+1:]...), true
		}
	}
	return items, false
}
