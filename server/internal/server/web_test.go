package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebHandlerServesAssetsAndSPAFallback(t *testing.T) {
	root := t.TempDir()
	writeWebTestFile(t, filepath.Join(root, "index.html"), "index")
	writeWebTestFile(t, filepath.Join(root, "assets", "app.js"), "asset")
	handler, err := NewWebHandler(root)
	require.NoError(t, err)

	tests := []struct {
		path         string
		status       int
		body         string
		cacheControl string
	}{
		{
			path:         "/assets/app.js",
			status:       http.StatusOK,
			body:         "asset",
			cacheControl: immutableCacheControl,
		},
		{
			path:         "/projects/example",
			status:       http.StatusOK,
			body:         "index",
			cacheControl: "no-cache",
		},
		{
			path:         "/projects/example/",
			status:       http.StatusOK,
			body:         "index",
			cacheControl: "no-cache",
		},
		{
			path:   "/assets/missing.js",
			status: http.StatusNotFound,
		},
		{
			path:   "/missing.js",
			status: http.StatusNotFound,
		},
		{
			path:   "/api/missing",
			status: http.StatusNotFound,
		},
		{
			path:   "/admin/missing",
			status: http.StatusNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.path == "/api/missing" {
				request = request.WithContext(contextWithAPIRequest(request))
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			assert.Equal(t, test.status, response.Code)
			if test.body != "" {
				assert.Equal(t, test.body, response.Body.String())
			}
			assert.Equal(t, test.cacheControl, response.Header().Get("Cache-Control"))
		})
	}
}

func TestWebHandlerRejectsInvalidRootAndTraversal(t *testing.T) {
	_, err := NewWebHandler(filepath.Join(t.TempDir(), "missing"))
	require.ErrorContains(t, err, "stat web root")

	root := t.TempDir()
	writeWebTestFile(t, filepath.Join(root, "index.html"), "index")
	handler, err := NewWebHandler(root)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/../secret", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func contextWithAPIRequest(request *http.Request) context.Context {
	return context.WithValue(request.Context(), apiRequestContextKey{}, true)
}

func writeWebTestFile(t *testing.T, filePath, contents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte(contents), 0o600))
}

func TestWebHandlerDoesNotFollowAssetSymlinksOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeWebTestFile(t, filepath.Join(root, "index.html"), "index")
	writeWebTestFile(t, filepath.Join(outside, "private.js"), "private-data")
	require.NoError(t, os.Mkdir(filepath.Join(root, "assets"), 0o750))
	require.NoError(t, os.Symlink(filepath.Join(outside, "private.js"), filepath.Join(root, "assets", "app.js")))
	handler, err := NewWebHandler(root)
	require.NoError(t, err)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	require.NotEqual(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), "private-data")
}

func TestWebHandlerRejectsIndexSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.html")
	writeWebTestFile(t, outside, "private-data")
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "index.html")))

	_, err := NewWebHandler(root)

	require.ErrorContains(t, err, "stat web index")
}
