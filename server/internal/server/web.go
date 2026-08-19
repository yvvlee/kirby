package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const immutableCacheControl = "public, max-age=31536000, immutable"

// NewWebHandler validates and serves a built single-page application.
// An empty root disables the web application for local API-only development.
func NewWebHandler(root string) (http.Handler, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve web root: %w", err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("stat web root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("web root is not a directory: %s", absoluteRoot)
	}
	indexPath := filepath.Join(absoluteRoot, "index.html")
	indexInfo, err := os.Stat(indexPath)
	if err != nil {
		return nil, fmt.Errorf("stat web index: %w", err)
	}
	if !indexInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("web index is not a regular file: %s", indexPath)
	}

	files := http.FileServer(http.Dir(absoluteRoot))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveWebRequest(writer, request, absoluteRoot, indexPath, files)
	}), nil
}

func serveWebRequest(writer http.ResponseWriter, request *http.Request, root, indexPath string, files http.Handler) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	cleanPath := path.Clean("/" + request.URL.Path)
	normalizedRequestPath := request.URL.Path
	if normalizedRequestPath != "/" {
		normalizedRequestPath = strings.TrimSuffix(normalizedRequestPath, "/")
	}
	if cleanPath != normalizedRequestPath || strings.ContainsRune(request.URL.Path, '\x00') {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if isAPIRequest(request) || reservedServerPath(cleanPath) {
		http.NotFound(writer, request)
		return
	}

	candidate := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(cleanPath, "/")))
	info, err := os.Stat(candidate)
	switch {
	case err == nil && info.Mode().IsRegular():
		if strings.HasPrefix(cleanPath, "/assets/") {
			writer.Header().Set("Cache-Control", immutableCacheControl)
		} else {
			writer.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(writer, request)
	case err != nil && !errors.Is(err, os.ErrNotExist):
		http.Error(writer, "unable to read web content", http.StatusInternalServerError)
	case strings.HasPrefix(cleanPath, "/assets/") || path.Ext(cleanPath) != "":
		http.NotFound(writer, request)
	default:
		writer.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(writer, request, indexPath)
	}
}

func reservedServerPath(requestPath string) bool {
	for _, prefix := range []string{"/admin", "/auth", "/healthz", "/readyz", "/v1"} {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}
