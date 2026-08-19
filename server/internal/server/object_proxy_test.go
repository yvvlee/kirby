package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/config"
)

func TestObjectStorageProxyPreservesPublicHostAndPath(t *testing.T) {
	var receivedHost, receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHost = request.Host
		receivedPath = request.URL.RequestURI()
		_, _ = io.WriteString(writer, "object")
	}))
	defer upstream.Close()

	proxy, prefix, err := newObjectStorageProxy(config.S3Config{
		Endpoint: upstream.Listener.Addr().String(),
		Bucket:   "kirby",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	assert.Equal(t, "/kirby", prefix)

	request := httptest.NewRequest(http.MethodGet, "http://public.example/kirby/file?signature=value", nil)
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "public.example", receivedHost)
	assert.Equal(t, "/kirby/file?signature=value", receivedPath)
}

func TestObjectStorageProxyRoutesDoNotRedirectUpload(t *testing.T) {
	server := kratoshttp.NewServer()
	registerObjectStorageProxy(server, "/kirby", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	for _, requestPath := range []string{"/kirby", "/kirby/", "/kirby/uploads/file"} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, requestPath, nil))
		assert.Equal(t, http.StatusNoContent, response.Code, requestPath)
		assert.Empty(t, response.Header().Get("Location"), requestPath)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/kirby-extra", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
}
