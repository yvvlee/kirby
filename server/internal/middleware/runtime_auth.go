package middleware

import (
	"context"
	"strings"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
)

const (
	HTTPHeader   = "X-Kirby-API-Key"
	GRPCMetadata = "x-kirby-api-key"
)

type credentialKey struct{}

// HTTPAPIKey copies the runtime credential from the HTTP header into context.
// Database verification remains in the runtime logic so authentication and the
// versioned read share one transaction.
//
// Parameters: next is the runtime handler.
// Returns: a handler carrying the credential in context.
// Throws: returns unauthorized if the credential is missing or ambiguous.
func HTTPAPIKey(next kratosmiddleware.Handler) kratosmiddleware.Handler {
	return func(ctx context.Context, request any) (any, error) {
		serverTransport, ok := transport.FromServerContext(ctx)
		if !ok {
			return nil, errorsv1.ErrorUnauthorized("API key authentication failed")
		}
		credential := serverTransport.RequestHeader().Get(HTTPHeader)
		if strings.TrimSpace(credential) == "" {
			return nil, errorsv1.ErrorUnauthorized("API key authentication failed")
		}
		return next(context.WithValue(ctx, credentialKey{}, credential), request)
	}
}

// UnaryAPIKey copies the runtime credential from gRPC metadata into context.
//
// Parameters: ctx carries metadata; request and info identify the call; handler executes it.
// Returns: the authenticated handler result.
// Throws: an authentication or downstream handler error.
func UnaryAPIKey(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if info != nil && !strings.HasPrefix(info.FullMethod, "/kirby.runtime.v1.Api/") {
		return handler(ctx, request)
	}
	values := metadata.ValueFromIncomingContext(ctx, GRPCMetadata)
	if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
		return nil, errorsv1.ErrorUnauthorized("API key authentication failed")
	}
	return handler(context.WithValue(ctx, credentialKey{}, values[0]), request)
}

// RuntimeCredentialFromContext returns the credential supplied by runtime authentication.
//
// Parameters: ctx is the runtime request context.
// Returns: the credential and whether one was attached.
// Throws: none.
func RuntimeCredentialFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(credentialKey{}).(string)
	return value, ok && value != ""
}
