package set

type Set[T comparable] struct{ values map[T]struct{} }

// New builds a set with duplicate inputs collapsed.
//
// Parameters: items are the initial members.
// Returns: a mutable set.
// Throws: none.
func New[T comparable](items ...T) *Set[T] {
	result := &Set[T]{values: make(map[T]struct{}, len(items))}
	result.Add(items...)
	return result
}

// Add makes repeated insertion idempotent.
//
// Parameters: items are the members to include.
// Returns: none.
// Throws: none.
func (s *Set[T]) Add(items ...T) {
	for _, item := range items {
		s.values[item] = struct{}{}
	}
}

// Remove treats absent members as already removed.
//
// Parameters: items are the members to exclude.
// Returns: none.
// Throws: none.
func (s *Set[T]) Remove(items ...T) {
	for _, item := range items {
		delete(s.values, item)
	}
}

// Contains tests membership without changing the set.
//
// Parameters: item is the candidate member.
// Returns: whether it is present.
// Throws: none.
func (s *Set[T]) Contains(item T) bool {
	_, ok := s.values[item]
	return ok
}

// Size reports distinct membership rather than insertion count.
//
// Parameters: none.
// Returns: the number of distinct members.
// Throws: none.
func (s *Set[T]) Size() int { return len(s.values) }

// Values returns a separate slice so callers cannot mutate membership.
//
// Parameters: none.
// Returns: the members in unspecified order.
// Throws: none.
func (s *Set[T]) Values() []T {
	result := make([]T, 0, len(s.values))
	for item := range s.values {
		result = append(result, item)
	}
	return result
}
