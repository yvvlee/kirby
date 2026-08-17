package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandlerServesAssetsAndSPAFallback(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "index")
	writeTestFile(t, filepath.Join(root, "assets", "app.js"), "asset")
	handler := newHandler(root)

	tests := []struct {
		path         string
		body         string
		cacheControl string
	}{
		{path: "/assets/app.js", body: "asset", cacheControl: "public, max-age=31536000, immutable"},
		{path: "/projects/example", body: "index", cacheControl: "no-cache"},
		{path: "/healthz", body: "ok\n", cacheControl: ""},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			if response.Body.String() != test.body {
				t.Fatalf("body = %q", response.Body.String())
			}
			if actual := response.Header().Get("Cache-Control"); actual != test.cacheControl {
				t.Fatalf("Cache-Control = %q", actual)
			}
		})
	}
}

func TestHandlerDoesNotEscapeContentRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.html"), "index")
	request := httptest.NewRequest(http.MethodGet, "/../secret", nil)
	response := httptest.NewRecorder()
	newHandler(root).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected traversal response: %d %q", response.Code, response.Body.String())
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
