package entity

import (
	"errors"
	"fmt"

	errorsv1 "github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

var (
	ErrInvalid  = errors.New("core domain input is invalid")
	ErrConflict = errors.New("core domain state conflicts")
)

func Invalid(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, arguments...))
}

func Conflict(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, arguments...))
}

func APIError(err error) error {
	switch {
	case errors.Is(err, ErrInvalid):
		return errorsv1.ErrorBadRequest("invalid configuration data")
	case errors.Is(err, ErrConflict), errors.Is(err, repository.ErrKeyConflict):
		return errorsv1.ErrorConflict("operation conflicts with current configuration")
	default:
		return permission.APIError(err)
	}
}
