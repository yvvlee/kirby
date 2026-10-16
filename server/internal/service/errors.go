package service

import (
	"errors"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

func apiError(err error) error {
	switch {
	case errors.Is(err, entity.ErrInvalid):
		return errorsv1.ErrorBadRequest("invalid configuration data")
	case errors.Is(err, entity.ErrConflict), errors.Is(err, repository.ErrKeyConflict):
		return errorsv1.ErrorConflict("operation conflicts with current configuration")
	default:
		return permission.APIError(err)
	}
}
