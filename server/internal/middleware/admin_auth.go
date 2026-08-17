package middleware

import (
	"context"
	"errors"
	"strings"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type principalKey struct{}

// Principal is the stable identity extracted from an access token. It has no
// environment or authorization claims.
type Principal struct {
	UserID    int64
	SessionID string
}

// PrincipalFromContext returns the authenticated administrator identity.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

type AccessTokenVerifier interface {
	Verify(string) (*authjwt.Claims, error)
}

type AuthenticatedUserReader interface {
	GetByID(context.Context, int64) (*model.User, error)
}

// AdminAuth validates access JWTs and current user status. A later selector
// must omit this middleware from login and refresh endpoints.
func AdminAuth(tokens AccessTokenVerifier, users AuthenticatedUserReader) kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			if tokens == nil || users == nil {
				return nil, errorsv1.ErrorInternal("authentication is unavailable")
			}
			serverTransport, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errorsv1.ErrorUnauthorized("authentication failed")
			}
			parts := strings.Fields(serverTransport.RequestHeader().Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return nil, errorsv1.ErrorUnauthorized("authentication failed")
			}
			claims, err := tokens.Verify(parts[1])
			if err != nil {
				return nil, errorsv1.ErrorUnauthorized("authentication failed")
			}
			userID, err := claims.UserID()
			if err != nil {
				return nil, errorsv1.ErrorUnauthorized("authentication failed")
			}
			user, err := users.GetByID(ctx, userID)
			if err != nil {
				if errors.Is(err, repository.ErrUserNotFound) {
					return nil, errorsv1.ErrorUnauthorized("authentication failed")
				}
				return nil, errorsv1.ErrorInternal("authentication is unavailable")
			}
			if !user.Enabled {
				return nil, errorsv1.ErrorUnauthorized("authentication failed")
			}
			ctx = context.WithValue(ctx, principalKey{}, Principal{UserID: userID, SessionID: claims.SessionID})
			return next(ctx, request)
		}
	}
}
