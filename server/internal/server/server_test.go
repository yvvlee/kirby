package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/service"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
)

type testRuntimeService struct {
	runtimev1.UnimplementedApiServer
}

func TestHTTPJSONUsesPublicSnakeCaseContract(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	err := kratoshttp.DefaultResponseEncoder(response, request, &adminv1.LoginReply{
		AccessToken: "token",
		ExpiresIn:   60,
	})
	require.NoError(t, err)
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"access_token":"token","expires_in":60,"user":null}`, response.Body.String())
}

func TestHTTPJSONIgnoresUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"access_token":"token","expires_in":60,"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	var reply adminv1.LoginReply
	require.NoError(t, kratoshttp.DefaultRequestDecoder(request, &reply))
	assert.Equal(t, "token", reply.AccessToken)
	assert.EqualValues(t, 60, reply.ExpiresIn)
}

func TestRegisterHTTPIncludesEveryBoundary(t *testing.T) {
	transport := kratoshttp.NewServer()
	registerHTTP(transport, &Dependencies{})
	routes := make(map[string]struct{})
	require.NoError(t, transport.WalkRoute(func(route kratoshttp.RouteInfo) error {
		routes[route.Method+" "+route.Path] = struct{}{}
		return nil
	}))
	for _, route := range []string{
		"POST /auth/login",
		"GET /setup/status",
		"POST /setup/admin",
		"GET /admin/environments",
		"POST /admin/environments/{environment_id}/projects/{project_id}/assets/presign",
		"POST /admin/environments/{environment_id}/snapshots/{snapshot_id}/publish",
		"POST /admin/environments/{target_environment_id}/snapshot-imports",
		"POST /admin/environments/{environment_id}/projects/{project_id}/api-keys",
		"GET /v1/config",
	} {
		_, ok := routes[route]
		assert.True(t, ok, "route not registered: %s", route)
	}
	assert.GreaterOrEqual(t, len(routes), 55)
}

func TestRuntimeGRPCServiceListExcludesKirbyAdmin(t *testing.T) {
	transport := newRuntimeGRPCServer(":0", time.Second, nil)
	registerGRPC(transport, testRuntimeService{})
	services := transport.ServiceInfo()
	require.Contains(t, services, "kirby.runtime.v1.Api")
	require.Contains(t, services, "grpc.health.v1.Health")
	for name := range services {
		assert.False(t, strings.HasPrefix(name, "kirby.admin.v1."), name)
		assert.Contains(t, []string{
			"kirby.runtime.v1.Api",
			"grpc.health.v1.Health",
			"grpc.reflection.v1.ServerReflection",
			"grpc.reflection.v1alpha.ServerReflection",
		}, name)
	}
}

func TestSameOriginNeedsNoDomainConfiguration(t *testing.T) {
	origins, err := authsession.NewOriginValidator(nil)
	require.NoError(t, err)
	handler := SameOrigin(origins)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	for _, origin := range []string{"", "https://kirby.example.com"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "https://kirby.example.com/api/auth/login", nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		handler.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, response.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestSameOriginRejectsCrossOriginAndOpaqueOrigins(t *testing.T) {
	origins, err := authsession.NewOriginValidator(nil)
	require.NoError(t, err)
	handler := SameOrigin(origins)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	for _, origin := range []string{
		"null",
		"https://kirby.example.com.evil",
		"http://kirby.example.com",
		"https://kirby.example.com:8443",
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodOptions, "https://kirby.example.com/api/auth/login", nil)
		request.Header.Set("Origin", origin)
		handler.ServeHTTP(response, request)
		assert.Equal(t, http.StatusForbidden, response.Code, origin)
		assert.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestHTTPBoundaryDoesNotLogQueryOrHeaders(t *testing.T) {
	var output bytes.Buffer
	logger, err := infra.NewLoggerWithWriter(&output, conf.LogConfig{Level: "info"})
	require.NoError(t, err)
	handler := HTTPBoundary(logger)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ok")
	}))
	request := httptest.NewRequest(http.MethodPut, "/api/assets/upload?token=secret-query", nil)
	request.Header.Set("Authorization", "Bearer secret-header")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.NotEmpty(t, response.Header().Get(requestIDHeader))
	assert.NotEmpty(t, response.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "no-referrer", response.Header().Get("Referrer-Policy"))
	assert.Equal(t, "DENY", response.Header().Get("X-Frame-Options"))
	assert.NotContains(t, output.String(), "secret-query")
	assert.NotContains(t, output.String(), "secret-header")
}

func TestAPIPrefixStripsGeneratedPathsAndPreservesLocalObjects(t *testing.T) {
	handler := APIPrefix()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.True(t, isAPIRequest(request))
		_, _ = io.WriteString(writer, request.URL.RequestURI())
	}))

	for _, test := range []struct {
		requestPath string
		wantPath    string
	}{
		{
			requestPath: "/api/auth/login?next=1",
			wantPath:    "/auth/login?next=1",
		},
		{
			requestPath: "/api/assets/upload?token=value",
			wantPath:    "/api/assets/upload?token=value",
		},
		{
			requestPath: "/api/assets/objects/example",
			wantPath:    "/api/assets/objects/example",
		},
	} {
		t.Run(test.requestPath, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.requestPath, nil))
			assert.Equal(t, test.wantPath, response.Body.String())
		})
	}
}

func TestAPIPrefixRejectsUnprefixedManagementRoutes(t *testing.T) {
	handler := APIPrefix()(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	for _, requestPath := range []string{"/auth/login", "/admin/users", "/setup/status", "/setup/admin"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/config", nil))
	assert.Equal(t, http.StatusNoContent, response.Code)
}

func TestSameOriginDefaultsToExternalHTTPSProxyAddress(t *testing.T) {
	origins, err := authsession.NewOriginValidator(nil)
	require.NoError(t, err)
	handler := SameOrigin(origins)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "http://kirby:8080/api/auth/login", nil)
	request.RemoteAddr = "172.20.0.2:12345"
	request.Header.Set("Origin", "https://kirby.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "kirby.example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
}

type serverSetupRepository struct {
	repository.SetupRepository
	initialized bool
}

func (r *serverSetupRepository) IsInitialized(context.Context) (bool, error) {
	return r.initialized, nil
}

func (r *serverSetupRepository) CreateInitialAdmin(context.Context, *model.User) error {
	if r.initialized {
		return repository.ErrAlreadyInitialized
	}
	r.initialized = true
	return nil
}

func TestSetupHTTPIsPublicAndClosesAfterInitialAdministrator(t *testing.T) {
	cfg := &conf.Config{
		HTTP: conf.ListenerConfig{
			Address: ":0",
			Timeout: conf.Duration{Duration: time.Second},
		},
	}
	logger, err := infra.NewLoggerWithWriter(io.Discard, conf.LogConfig{Level: "info"})
	require.NoError(t, err)
	store := &infra.Storage{
		Cache: cache.NewMemory(),
	}
	setupLogic := logic.NewSetupLogic(&serverSetupRepository{}, password.NewDefault(), serverTransactions{})
	setupService, err := service.NewSetupService(cfg, setupLogic)
	require.NoError(t, err)
	server, err := NewHTTPServer(&Dependencies{
		Config: cfg,
		Logger: logger,
		Store:  store,
		Setup:  setupService,
	}, nil, nil)
	require.NoError(t, err)

	status := httptest.NewRecorder()
	server.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "https://kirby.example.com/api/setup/status", nil))
	assert.Equal(t, http.StatusOK, status.Code)
	assert.JSONEq(t, `{"initialized":false}`, status.Body.String())

	for _, expected := range []int{http.StatusOK, http.StatusConflict} {
		request := httptest.NewRequest(http.MethodPost, "https://kirby.example.com/api/setup/admin", strings.NewReader(`{"username":"owner","display_name":"Owner","password":"initial-password"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://kirby.example.com")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		assert.Equal(t, expected, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("Set-Cookie"))
	}

	status = httptest.NewRecorder()
	server.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "https://kirby.example.com/api/setup/status", nil))
	assert.Equal(t, http.StatusOK, status.Code)
	assert.JSONEq(t, `{"initialized":true}`, status.Body.String())
}

func TestAPIPrefixAllowsSetupPageAndRequiresAPINamespaceForSetupEndpoints(t *testing.T) {
	handler := APIPrefix()(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(writer, request.URL.Path)
	}))
	for _, path := range []string{"/setup", "/setup/"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusAccepted, response.Code, path)
		assert.Equal(t, path, response.Body.String())
	}
	for _, path := range []string{"/setup/status", "/setup/status/", "/setup/admin", "/setup/admin/"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusNotFound, response.Code, path)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/setup/status", nil))
	assert.Equal(t, http.StatusAccepted, response.Code)
	assert.Equal(t, "/setup/status", response.Body.String())
}

type exhaustedSetupRateCache struct {
	cache.Store
}

func (exhaustedSetupRateCache) Increment(context.Context, string, time.Duration) (int64, error) {
	return authRateLimit + 1, nil
}

func TestInitialAdministratorCreationUsesAuthenticationRateLimit(t *testing.T) {
	cfg := &conf.Config{
		HTTP: conf.ListenerConfig{
			Address: ":0",
			Timeout: conf.Duration{Duration: time.Second},
		},
	}
	logger, err := infra.NewLoggerWithWriter(io.Discard, conf.LogConfig{Level: "info"})
	require.NoError(t, err)
	server, err := NewHTTPServer(&Dependencies{
		Config: cfg,
		Logger: logger,
		Store: &infra.Storage{
			Cache: exhaustedSetupRateCache{Store: cache.NewMemory()},
		},
	}, nil, nil)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "https://kirby.example.com/api/setup/admin", strings.NewReader(`{"username":"owner","display_name":"Owner","password":"initial-password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://kirby.example.com")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	assert.Equal(t, http.StatusTooManyRequests, response.Code, response.Body.String())
}
