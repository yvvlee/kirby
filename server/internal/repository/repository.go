package repository

import (
	"context"
	"errors"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
)

// WrapDatabaseError preserves cancellation and converts storage failures to API errors.
// Parameters: err is the original database error.
// Returns: nil, the cancellation error, or a database error retaining its cause.
// Throws: none.
func WrapDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errorsv1.ErrorDatabaseError("%s", err.Error()).WithCause(err)
}

func modelValues[T any](rows []*T) []T {
	values := make([]T, len(rows))
	for index, row := range rows {
		values[index] = *row
	}
	return values
}
