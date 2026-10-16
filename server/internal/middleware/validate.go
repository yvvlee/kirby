package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
)

type validatedRequest interface {
	Validate() error
}

// ValidateRequest preserves the generated PGV rules on both HTTP and gRPC requests.
//
// Parameters: none.
// Returns: middleware that validates before invoking the next handler.
// Throws: returns VALIDATOR bad-request errors with their original validation cause.
func ValidateRequest() kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			if err := validateRequest(request); err != nil {
				return nil, err
			}
			return next(ctx, request)
		}
	}
}

func validateRequest(request any) error {
	validator, ok := request.(validatedRequest)
	if !ok {
		return nil
	}
	if err := validator.Validate(); err != nil {
		return errors.BadRequest("VALIDATOR", err.Error()).WithCause(err)
	}
	return nil
}
