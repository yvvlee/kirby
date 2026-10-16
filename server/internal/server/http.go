package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/health"
	"github.com/yvvlee/kirby/server/internal/infra/object"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
)

// NewHTTPServer registers management, setup, runtime, health, and web routes.
// Parameters: deps supplies services; webHandler serves the browser app; probe reports health.
// Returns: the configured HTTP transport.
// Throws: errors for invalid proxy policy or object storage proxy configuration.
func NewHTTPServer(deps *Dependencies, webHandler http.Handler, probe *health.Probe) (*kratoshttp.Server, error) {
	origins, err := authsession.NewOriginValidator(deps.Config.Security.TrustedProxies)
	if err != nil {
		return nil, err
	}
	adminAuth := adminmiddleware.AdminAuth(deps.Tokens, deps.Users)
	adminRate := adminmiddleware.FixedWindowRateLimit(deps.Store.Cache, "admin", adminRateLimit, rateWindow, adminmiddleware.AuthorizationIdentity)
	authRate := adminmiddleware.FixedWindowRateLimit(deps.Store.Cache, "auth", authRateLimit, rateWindow, adminmiddleware.ClientIdentity)
	runtimeRate := adminmiddleware.FixedWindowRateLimit(deps.Store.Cache, "runtime", runtimeRateLimit, rateWindow, adminmiddleware.RuntimeIdentity)

	adminSelector := selector.Server(adminRate, adminAuth).Match(func(_ context.Context, operation string) bool {
		return strings.HasPrefix(operation, "/kirby.admin.v1.") && !publicManagementOperation(operation)
	}).Build()
	authSelector := selector.Server(authRate).Path(adminv1.OperationAuthServiceLogin, adminv1.OperationAuthServiceRefresh, adminv1.OperationSetupServiceCreateInitialAdmin).Build()
	runtimeHTTPSelector := selector.Server(runtimeRate, adminmiddleware.HTTPAPIKey).Path(runtimev1.OperationApiConfig).Build()

	httpServer := kratoshttp.NewServer(
		kratoshttp.Address(deps.Config.HTTP.Address),
		kratoshttp.Timeout(deps.Config.HTTP.Timeout.Duration),
		kratoshttp.Filter(HTTPBoundary(deps.Logger), APIPrefix(), SameOrigin(origins)),
		kratoshttp.Middleware(recovery.Recovery(), adminmiddleware.ValidateRequest(), authSelector, adminSelector, runtimeHTTPSelector),
	)
	registerHTTP(httpServer, deps)
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

	return httpServer, nil
}

func publicManagementOperation(operation string) bool {
	switch operation {
	case adminv1.OperationAuthServiceLogin,
		adminv1.OperationAuthServiceRefresh,
		adminv1.OperationAuthServiceLogout,
		adminv1.OperationSetupServiceGetSetupStatus,
		adminv1.OperationSetupServiceCreateInitialAdmin:
		return true
	default:
		return false
	}
}

func registerObjectStorageProxy(server *kratoshttp.Server, prefix string, handler http.Handler) {
	// Prefix must be registered first. Kratos uses StrictSlash, so placing the
	// exact route first would redirect POST /bucket/ to GET /bucket.
	server.HandlePrefix(prefix+"/", handler)
	server.Handle(prefix, handler)
}

func registerHTTP(server *kratoshttp.Server, deps *Dependencies) {
	adminv1.RegisterAuthServiceHTTPServer(server, deps.Auth)
	adminv1.RegisterSetupServiceHTTPServer(server, deps.Setup)
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
