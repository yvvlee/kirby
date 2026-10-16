package repository

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound deliberately covers both a missing row and a row that belongs
	// to another environment. This prevents repositories from leaking whether a
	// foreign-environment resource exists.
	ErrNotFound = errors.New("repository: resource not found")
	// ErrNoRowsAffected reports a failed guarded write. Callers may distinguish
	// a stale version from a missing resource by doing a scoped read first.
	ErrNoRowsAffected  = errors.New("repository: no rows affected")
	ErrInvalidArgument = errors.New("repository: invalid argument")
)

// InvalidArgument returns a repository error that callers can classify.
// Parameters: name identifies the invalid field.
// Returns: an error wrapping ErrInvalidArgument.
// Throws: none.
func InvalidArgument(name string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, name)
}

// Missing returns a repository error that callers can classify.
// Parameters: resource identifies the inaccessible resource.
// Returns: an error wrapping ErrNotFound.
// Throws: none.
func Missing(resource string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, resource)
}

// Unchanged returns a repository error that callers can classify.
// Parameters: resource identifies the guarded write resource.
// Returns: an error wrapping ErrNoRowsAffected.
// Throws: none.
func Unchanged(resource string) error {
	return fmt.Errorf("%w: %s", ErrNoRowsAffected, resource)
}
