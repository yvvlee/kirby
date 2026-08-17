package permission

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"

	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
)

type Actor struct {
	UserID    int64
	RequestID string
}

func ActorFromContext(ctx context.Context) (Actor, error) {
	principal, ok := adminmiddleware.PrincipalFromContext(ctx)
	if !ok || principal.UserID <= 0 {
		return Actor{}, ErrForbidden
	}
	requestID := ""
	if serverTransport, ok := transport.FromServerContext(ctx); ok {
		requestID = serverTransport.RequestHeader().Get("X-Request-ID")
	}
	if !validRequestID(requestID) {
		requestID = uuid.NewString()
	}
	return Actor{UserID: principal.UserID, RequestID: requestID}, nil
}

func validRequestID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
			continue
		}
		switch character {
		case '.', '_', ':', '-':
			continue
		default:
			return false
		}
	}
	return true
}
