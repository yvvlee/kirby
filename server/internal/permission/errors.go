package permission

import (
	"errors"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

// APIError converts internal authorization and persistence classes without
// exposing database details or foreign-environment resource existence.
func APIError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrForbidden):
		return errorsv1.ErrorForbidden("permission denied")
	case errors.Is(err, ErrEnvironmentNotFound), errors.Is(err, base.ErrNotFound), errors.Is(err, repository.ErrUserNotFound):
		return errorsv1.ErrorNotFound("resource not found")
	case errors.Is(err, base.ErrInvalidArgument):
		return errorsv1.ErrorBadRequest("invalid request")
	case errors.Is(err, base.ErrNoRowsAffected),
		errors.Is(err, repository.ErrUserVersionConflict),
		errors.Is(err, repository.ErrLastSystemAdmin),
		errors.Is(err, repository.ErrRoleInUse),
		errors.Is(err, repository.ErrBuiltinRole),
		errors.Is(err, repository.ErrSystemRolePermission),
		errors.Is(err, repository.ErrPermissionSetMismatch),
		errors.Is(err, repository.ErrUserListLimit),
		errors.Is(err, ErrConcurrentChange):
		return errorsv1.ErrorConflict("operation conflicts with current state")
	default:
		return errorsv1.ErrorInternal("operation failed")
	}
}
