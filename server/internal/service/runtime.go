package service

import (
	"context"
	"errors"

	"github.com/yvvlee/kirby/server/internal/infra"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type runtimeLogic interface {
	Read(ctx context.Context, fullCredential, requestedProject, configKey string) (*logic.RuntimeResult, error)
}

type RuntimeService struct {
	runtimev1.UnimplementedApiServer
	runtimeLogic runtimeLogic
	logger       *infra.Logger
}

// NewRuntimeService creates RuntimeService with its application operations.
//
// Parameters: logicLayer supplies the application operations; logger records operational failures.
// Returns: *RuntimeService.
// Throws: none.
func NewRuntimeService(logicLayer *logic.RuntimeLogic, logger *infra.Logger) *RuntimeService {
	return &RuntimeService{
		runtimeLogic: logicLayer,
		logger:       logger,
	}
}

var (
	_ runtimev1.ApiHTTPServer = (*RuntimeService)(nil)
	_ runtimev1.ApiServer     = (*RuntimeService)(nil)
)

// Config maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *runtimev1.ConfigReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *RuntimeService) Config(ctx context.Context, request *runtimev1.ConfigRequest) (*runtimev1.ConfigReply, error) {
	credential, ok := middleware.RuntimeCredentialFromContext(ctx)
	if !ok {
		return nil, errorsv1.ErrorUnauthorized("API key authentication failed")
	}
	result, err := s.runtimeLogic.Read(ctx, credential, request.Project, request.Key)
	if err != nil {
		if isInternalError(err) {
			s.logger.WithContext(ctx).Errorw("msg", "runtime config read failed", "error", err)
		}
		return nil, runtimePublicError(err)
	}
	if result == nil {
		return nil, errorsv1.ErrorInternal("runtime config read failed")
	}
	return &runtimev1.ConfigReply{
		Content: result.Content,
		Version: result.Version,
	}, nil
}

func isInternalError(err error) bool {
	return !errors.Is(err, logic.ErrUnauthenticated) &&
		!errors.Is(err, logic.ErrProjectMismatch) &&
		!errors.Is(err, repository.ErrInvalidArgument) &&
		!errors.Is(err, repository.ErrNotFound)
}

func runtimePublicError(err error) error {
	switch {
	case errors.Is(err, logic.ErrUnauthenticated):
		return errorsv1.ErrorUnauthorized("API key authentication failed")
	case errors.Is(err, logic.ErrProjectMismatch):
		return errorsv1.ErrorForbidden("API key does not belong to requested project")
	case errors.Is(err, repository.ErrInvalidArgument):
		return errorsv1.ErrorBadRequest("invalid runtime config request")
	case errors.Is(err, repository.ErrNotFound):
		return errorsv1.ErrorNotFound("runtime config was not found")
	default:
		return errorsv1.ErrorInternal("runtime config read failed")
	}
}
