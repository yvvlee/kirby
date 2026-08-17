// Package base contains repository primitives that do not expose unscoped
// resource access.
package base

import (
	"context"
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

func InvalidArgument(name string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, name)
}

func ValidateID(name string, value int64) error {
	if value <= 0 {
		return InvalidArgument(name + " must be greater than zero")
	}
	return nil
}

func ValidateEngine(engineNil bool) error {
	if engineNil {
		return InvalidArgument("database engine is nil")
	}
	return nil
}

func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("repository %s: %w", op, err)
}

func Missing(resource string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, resource)
}

func Unchanged(resource string) error {
	return fmt.Errorf("%w: %s", ErrNoRowsAffected, resource)
}
