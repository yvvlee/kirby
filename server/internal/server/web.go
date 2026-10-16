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
//
// Parameters: root selects the web directory; an empty value disables browser serving.
// Returns: the HTTP handler, or nil when disabled.
// Throws: an error for an inaccessible root or missing regular index file.
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
	directory, err := os.OpenRoot(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("open web root: %w", err)
	}
	defer directory.Close()
	indexInfo, err := directory.Stat("index.html")
	if err != nil {
		return nil, fmt.Errorf("stat web index: %w", err)
	}
	if !indexInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("web index is not a regular file")
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveWebRequest(writer, request, absoluteRoot)
	}), nil
}

func serveWebRequest(writer http.ResponseWriter, request *http.Request, root string) {
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

	// Root confines stat and file serving even if a symlink changes during a request.
	directory, err := os.OpenRoot(root)
	if err != nil {
		http.Error(writer, "unable to read web content", http.StatusInternalServerError)
		return
	}
	defer directory.Close()
	candidate := strings.TrimPrefix(cleanPath, "/")
	if candidate == "" {
		candidate = "."
	}
	info, err := directory.Stat(candidate)
	switch {
	case err == nil && info.Mode().IsRegular():
		if strings.HasPrefix(cleanPath, "/assets/") {
			writer.Header().Set("Cache-Control", immutableCacheControl)
		} else {
			writer.Header().Set("Cache-Control", "no-cache")
		}
		http.FileServerFS(directory.FS()).ServeHTTP(writer, request)
	case err != nil && !errors.Is(err, os.ErrNotExist):
		http.Error(writer, "unable to read web content", http.StatusInternalServerError)
	case strings.HasPrefix(cleanPath, "/assets/") || path.Ext(cleanPath) != "":
		http.NotFound(writer, request)
	default:
		writer.Header().Set("Cache-Control", "no-cache")
		serveWebIndex(writer, request, directory)
	}
}

func serveWebIndex(writer http.ResponseWriter, request *http.Request, directory *os.Root) {
	index, err := directory.Open("index.html")
	if err != nil {
		http.Error(writer, "unable to read web content", http.StatusInternalServerError)
		return
	}
	defer index.Close()
	info, err := index.Stat()
	if err != nil {
		http.Error(writer, "unable to read web content", http.StatusInternalServerError)
		return
	}
	http.ServeContent(writer, request, "index.html", info.ModTime(), index)
}

func reservedServerPath(requestPath string) bool {
	for _, prefix := range []string{"/admin", "/auth", "/healthz", "/readyz", "/v1"} {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}
