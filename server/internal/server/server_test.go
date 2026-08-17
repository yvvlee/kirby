package server

import (
	"bytes"
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

	runtimev1 "github.com/yvvlee/kirby/server/gen/kirby/runtime/v1"
	"github.com/yvvlee/kirby/server/internal/provider"
)

type testRuntimeService struct {
	runtimev1.UnimplementedApiServer
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
	assert.NotContains(t, output.String(), "secret-query")
	assert.NotContains(t, output.String(), "secret-header")
}
