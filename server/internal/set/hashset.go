package set

type Set[T comparable] struct{ values map[T]struct{} }

func New[T comparable](items ...T) *Set[T] {
	result := &Set[T]{values: make(map[T]struct{}, len(items))}
	result.Add(items...)
	return result
}

func (s *Set[T]) Add(items ...T) {
	for _, item := range items {
		s.values[item] = struct{}{}
	}
}

func (s *Set[T]) Remove(items ...T) {
	for _, item := range items {
		delete(s.values, item)
	}
}

func (s *Set[T]) Contains(item T) bool {
	_, ok := s.values[item]
	return ok
}

func (s *Set[T]) Size() int { return len(s.values) }

func (s *Set[T]) Values() []T {
	result := make([]T, 0, len(s.values))
	for item := range s.values {
		result = append(result, item)
	}
	return result
}
