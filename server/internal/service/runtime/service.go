package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	errorsv1 "github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	runtimev1 "github.com/yvvlee/kirby/server/gen/kirby/runtime/v1"
	logic "github.com/yvvlee/kirby/server/internal/logic/runtime"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

const (
	HTTPHeader   = "X-Kirby-API-Key"
	GRPCMetadata = "x-kirby-api-key"
)

type credentialKey struct{}

type Logic interface {
	Read(context.Context, string, string, string) (*logic.Result, error)
}

type Service struct {
	runtimev1.UnimplementedApiServer
	logic  Logic
	logger *slog.Logger
}

func New(logicLayer *logic.Logic, logger *slog.Logger) (*Service, error) {
	if logicLayer == nil || logger == nil {
		return nil, fmt.Errorf("runtime service logic and logger are required")
	}
	return &Service{logic: logicLayer, logger: logger}, nil
}

var (
	_ runtimev1.ApiHTTPServer = (*Service)(nil)
	_ runtimev1.ApiServer     = (*Service)(nil)
)

func (s *Service) Config(ctx context.Context, request *runtimev1.ConfigRequest) (*runtimev1.ConfigReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid runtime config request")
	}
	credential, ok := credentialFromContext(ctx)
	if !ok {
		return nil, errorsv1.ErrorUnauthorized("API key authentication failed")
	}
	result, err := s.logic.Read(ctx, credential, request.Project, request.Key)
	if err != nil {
		if s.logger != nil && isInternalError(err) {
			s.logger.ErrorContext(ctx, "runtime config read failed", "error", err)
		}
		return nil, publicError(err)
	}
	if result == nil {
		return nil, errorsv1.ErrorInternal("runtime config read failed")
	}
	return &runtimev1.ConfigReply{Content: result.Content, Version: result.Version}, nil
}

func isInternalError(err error) bool {
	return !errors.Is(err, logic.ErrUnauthenticated) &&
		!errors.Is(err, logic.ErrProjectMismatch) &&
		!errors.Is(err, base.ErrInvalidArgument) &&
		!errors.Is(err, base.ErrNotFound)
}

// HTTPAPIKey copies the runtime credential from the HTTP header into context.
// Database verification remains in Logic.Read so authentication and the
// versioned read share one transaction.
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

func credentialFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(credentialKey{}).(string)
	return value, ok && value != ""
}

func publicError(err error) error {
	switch {
	case errors.Is(err, logic.ErrUnauthenticated):
		return errorsv1.ErrorUnauthorized("API key authentication failed")
	case errors.Is(err, logic.ErrProjectMismatch):
		return errorsv1.ErrorForbidden("API key does not belong to requested project")
	case errors.Is(err, base.ErrInvalidArgument):
		return errorsv1.ErrorBadRequest("invalid runtime config request")
	case errors.Is(err, base.ErrNotFound):
		return errorsv1.ErrorNotFound("runtime config was not found")
	default:
		return errorsv1.ErrorInternal("runtime config read failed")
	}
}
