package entity

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid  = errors.New("core domain input is invalid")
	ErrConflict = errors.New("core domain state conflicts")
)

// Invalid marks a domain validation failure for transport-layer conversion.
//
// Parameters: format and arguments describe the invalid value without transport details.
// Returns: an error wrapping ErrInvalid.
// Throws: none.
func Invalid(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, arguments...))
}

// Conflict marks a domain constraint failure for transport-layer conversion.
//
// Parameters: format and arguments describe the violated constraint.
// Returns: an error wrapping ErrConflict.
// Throws: none.
func Conflict(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, arguments...))
}
