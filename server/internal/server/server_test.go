package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
	"github.com/yvvlee/kirby/server/internal/provider"
)

type testRuntimeService struct {
	runtimev1.UnimplementedApiServer
}

func TestGeneratedJSONUsesPublicSnakeCaseContract(t *testing.T) {
	encoded, err := json.Marshal(&adminv1.LoginReply{AccessToken: "token", ExpiresIn: 60})
	require.NoError(t, err)
	assert.JSONEq(t, `{"access_token":"token","expires_in":60,"user":null}`, string(encoded))
}

func TestRegisterHTTPIncludesEveryBoundary(t *testing.T) {
	transport := kratoshttp.NewServer()
	registerHTTP(transport, &provider.Application{})
	routes := make(map[string]struct{})
	require.NoError(t, transport.WalkRoute(func(route kratoshttp.RouteInfo) error {
		routes[route.Method+" "+route.Path] = struct{}{}
		return nil
	}))
	for _, route := range []string{
		"POST /auth/login",
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

func TestCORSRejectsUnknownOriginAndAllowsExactOrigin(t *testing.T) {
	handler := CORS([]string{"https://kirby.example.com"})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	denied := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	request.Header.Set("Origin", "https://kirby.example.com.evil")
	handler.ServeHTTP(denied, request)
	assert.Equal(t, http.StatusForbidden, denied.Code)
	assert.Empty(t, denied.Header().Get("Access-Control-Allow-Origin"))

	allowed := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	request.Header.Set("Origin", "https://kirby.example.com")
	handler.ServeHTTP(allowed, request)
	assert.Equal(t, http.StatusNoContent, allowed.Code)
	assert.Equal(t, "https://kirby.example.com", allowed.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", allowed.Header().Get("Access-Control-Allow-Credentials"))
}

func TestHTTPBoundaryDoesNotLogQueryOrHeaders(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
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
		{requestPath: "/api/auth/login?next=1", wantPath: "/auth/login?next=1"},
		{requestPath: "/api/assets/upload?token=value", wantPath: "/api/assets/upload?token=value"},
		{requestPath: "/api/assets/objects/example", wantPath: "/api/assets/objects/example"},
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

	for _, requestPath := range []string{"/auth/login", "/admin/users"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/config", nil))
	assert.Equal(t, http.StatusNoContent, response.Code)
}
