// Package server exposes Kirby's management HTTP and runtime HTTP/gRPC transports.
package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
	"github.com/yvvlee/kirby/server/internal/health"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/provider"
	runtimeauth "github.com/yvvlee/kirby/server/internal/service/runtime"
	"github.com/yvvlee/kirby/server/internal/storage/object"
	"github.com/yvvlee/kirby/server/internal/version"
)

const (
	adminRateLimit   = 600
	authRateLimit    = 60
	runtimeRateLimit = 1200
	rateWindow       = time.Minute
)

// NewApplication registers every public transport before any listener starts.
func NewApplication(ctx context.Context, deps *provider.Application, webHandler http.Handler) (*kratos.App, error) {
	if deps == nil || deps.Config == nil || deps.Store == nil || deps.Runtime == nil {
		return nil, fmt.Errorf("server dependencies are incomplete")
	}
	adminAuth := adminmiddleware.AdminAuth(deps.Tokens, deps.Users)
	adminRate := adminmiddleware.FixedWindowRateLimit(deps.Store.Cache, "admin", adminRateLimit, rateWindow, adminmiddleware.AuthorizationIdentity)
	authRate := adminmiddleware.FixedWindowRateLimit(deps.Store.Cache, "auth", authRateLimit, rateWindow, adminmiddleware.ClientIdentity)
	runtimeRate := adminmiddleware.FixedWindowRateLimit(deps.Store.Cache, "runtime", runtimeRateLimit, rateWindow, adminmiddleware.RuntimeIdentity)

	adminSelector := selector.Server(adminRate, adminAuth).Match(func(_ context.Context, operation string) bool {
		return strings.HasPrefix(operation, "/kirby.admin.v1.") && operation != adminv1.OperationAuthServiceLogin && operation != adminv1.OperationAuthServiceRefresh && operation != adminv1.OperationAuthServiceLogout
	}).Build()
	authSelector := selector.Server(authRate).Path(adminv1.OperationAuthServiceLogin, adminv1.OperationAuthServiceRefresh).Build()
	runtimeHTTPSelector := selector.Server(runtimeRate, runtimeauth.HTTPAPIKey).Path(runtimev1.OperationApiConfig).Build()
	runtimeGRPCSelector := selector.Server(runtimeRate).Path(runtimev1.OperationApiConfig).Build()

	httpServer := kratoshttp.NewServer(
		kratoshttp.Address(deps.Config.HTTP.Address),
		kratoshttp.Timeout(deps.Config.HTTP.Timeout.Duration),
		kratoshttp.Filter(HTTPBoundary(deps.Logger), APIPrefix(), CORS(deps.Config.Security.AllowedOrigins)),
		kratoshttp.Middleware(recovery.Recovery(), authSelector, adminSelector, runtimeHTTPSelector),
	)
	registerHTTP(httpServer, deps)
	probe := health.NewProbe()
	health.Register(httpServer, probe)
	if local, ok := deps.Object.(*object.LocalStorage); ok {
		httpServer.Handle(object.LocalUploadPath, local)
		httpServer.HandlePrefix(object.LocalObjectPathPrefix, local)
	}
	if deps.Config.ObjectStorage.Driver == "s3" {
		proxy, prefix, err := newObjectStorageProxy(deps.Config.ObjectStorage.S3, deps.Logger)
		if err != nil {
			return nil, err
		}
		registerObjectStorageProxy(httpServer, prefix, proxy)
	}
	if webHandler != nil {
		httpServer.HandlePrefix("/", webHandler)
	}

	grpcServer := newRuntimeGRPCServer(
		deps.Config.GRPC.Address,
		deps.Config.GRPC.Timeout.Duration,
		[]middleware.Middleware{recovery.Recovery(), adminmiddleware.TransportLog(deps.Logger), runtimeGRPCSelector},
		runtimeauth.UnaryAPIKey,
	)
	registerGRPC(grpcServer, deps.Runtime)

	return kratos.New(
		kratos.Name("kirby"), kratos.Version(version.Version), kratos.Context(ctx),
		kratos.Server(httpServer, grpcServer),
		kratos.AfterStart(func(context.Context) error { probe.SetReady(true); return nil }),
		kratos.BeforeStop(func(context.Context) error { probe.SetReady(false); return nil }),
	), nil
}

func registerObjectStorageProxy(server *kratoshttp.Server, prefix string, handler http.Handler) {
	// Prefix must be registered first. Kratos uses StrictSlash, so placing the
	// exact route first would redirect POST /bucket/ to GET /bucket.
	server.HandlePrefix(prefix+"/", handler)
	server.Handle(prefix, handler)
}

func registerGRPC(server *runtimeGRPCServer, runtimeService runtimev1.ApiServer) {
	runtimev1.RegisterApiServer(server.server, runtimeService)
}

func registerHTTP(server *kratoshttp.Server, deps *provider.Application) {
	adminv1.RegisterAuthServiceHTTPServer(server, deps.Auth)
	adminv1.RegisterEnvironmentServiceHTTPServer(server, deps.Environments)
	adminv1.RegisterUserServiceHTTPServer(server, deps.UsersService)
	adminv1.RegisterRoleServiceHTTPServer(server, deps.Roles)
	adminv1.RegisterProjectServiceHTTPServer(server, deps.Projects)
	adminv1.RegisterConfigServiceHTTPServer(server, deps.Configs)
	adminv1.RegisterStructureServiceHTTPServer(server, deps.Structures)
	adminv1.RegisterEnumServiceHTTPServer(server, deps.Enums)
	adminv1.RegisterSnapshotServiceHTTPServer(server, deps.Snapshots)
	adminv1.RegisterPublicationServiceHTTPServer(server, deps.Publications)
	adminv1.RegisterAssetServiceHTTPServer(server, deps.Assets)
	adminv1.RegisterProjectApiKeyServiceHTTPServer(server, deps.APIKeys)
	adminv1.RegisterSnapshotTransferServiceHTTPServer(server, deps.Transfers)
	runtimev1.RegisterApiHTTPServer(server, deps.Runtime)
}
