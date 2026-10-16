package repository

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type PageRequest struct {
	Offset int
	Limit  int
}

type PageResult[T any] struct {
	Items  []T
	Total  int64
	Offset int
	Limit  int
}

// NormalizePage applies the shared pagination defaults and upper bound.
// Parameters: page contains the requested offset and limit.
// Returns: a normalized page request.
// Throws: none.
func NormalizePage(page PageRequest) PageRequest {
	if page.Offset < 0 {
		page.Offset = 0
	}
	if page.Limit <= 0 {
		page.Limit = DefaultPageSize
	}
	if page.Limit > MaxPageSize {
		page.Limit = MaxPageSize
	}
	return page
}
