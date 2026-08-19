package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/yvvlee/kirby/server/internal/config"
)

func newObjectStorageProxy(cfg config.S3Config, logger *slog.Logger) (http.Handler, string, error) {
	if logger == nil {
		return nil, "", fmt.Errorf("object storage proxy logger is nil")
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	target, err := url.Parse(scheme + "://" + cfg.Endpoint)
	if err != nil {
		return nil, "", fmt.Errorf("parse object storage proxy target: %w", err)
	}
	prefix := "/" + strings.Trim(cfg.Bucket, "/")
	if prefix == "/" {
		return nil, "", fmt.Errorf("object storage bucket is empty")
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, _ error) {
		logger.Error("object storage proxy request failed", "request_id", request.Header.Get(requestIDHeader), "method", request.Method, "path", request.URL.Path)
		http.Error(writer, "object storage is unavailable", http.StatusBadGateway)
	}
	return proxy, prefix, nil
}
