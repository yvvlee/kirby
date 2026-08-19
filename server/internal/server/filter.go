package server

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

type apiRequestContextKey struct{}

// APIPrefix preserves the browser-facing /api namespace while generated
// handlers continue to use their protobuf-defined paths.
func APIPrefix() kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/api" && !strings.HasPrefix(request.URL.Path, "/api/") {
				if reservedManagementPath(request.URL.Path) {
					http.NotFound(writer, request)
					return
				}
				next.ServeHTTP(writer, request)
				return
			}
			request = request.WithContext(context.WithValue(request.Context(), apiRequestContextKey{}, true))
			if request.URL.Path == "/api" || isLocalObjectPath(request.URL.Path) {
				next.ServeHTTP(writer, request)
				return
			}
			cloned := request.Clone(request.Context())
			cloned.URL.Path = strings.TrimPrefix(request.URL.Path, "/api")
			cloned.URL.RawPath = ""
			next.ServeHTTP(writer, cloned)
		})
	}
}

func reservedManagementPath(requestPath string) bool {
	for _, prefix := range []string{"/admin", "/auth"} {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}

func isLocalObjectPath(requestPath string) bool {
	return requestPath == "/api/assets/upload" || strings.HasPrefix(requestPath, "/api/assets/objects/")
}

func isAPIRequest(request *http.Request) bool {
	value, _ := request.Context().Value(apiRequestContextKey{}).(bool)
	return value
}

func HTTPBoundary(logger *slog.Logger) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requestID := request.Header.Get(requestIDHeader)
			if !validRequestID(requestID) {
				requestID = uuid.NewString()
			}
			request.Header.Set(requestIDHeader, requestID)
			writer.Header().Set(requestIDHeader, requestID)
			writer.Header().Set("X-Content-Type-Options", "nosniff")
			writer.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:; form-action 'self'")
			writer.Header().Set("Referrer-Policy", "no-referrer")
			writer.Header().Set("X-Frame-Options", "DENY")
			statusWriter := &responseStatusWriter{ResponseWriter: writer, status: http.StatusOK}
			started := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("HTTP handler panic", "request_id", requestID, "method", request.Method, "path", request.URL.Path, "stack", string(debug.Stack()))
					if !statusWriter.wroteHeader {
						http.Error(statusWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
				logger.Info("HTTP request", "request_id", requestID, "method", request.Method, "path", request.URL.Path, "status", statusWriter.status, "duration", time.Since(started))
			}()
			next.ServeHTTP(statusWriter, request)
		})
	}
}

func CORS(allowedOrigins []string) kratoshttp.FilterFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(writer, request)
				return
			}
			if _, ok := allowed[origin]; !ok {
				http.Error(writer, "origin is not allowed", http.StatusForbidden)
				return
			}
			writer.Header().Add("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Credentials", "true")
			if request.Method == http.MethodOptions {
				method := request.Header.Get("Access-Control-Request-Method")
				if !allowedMethod(method) || !allowedHeaders(request.Header.Get("Access-Control-Request-Headers")) {
					http.Error(writer, "CORS preflight is not allowed", http.StatusForbidden)
					return
				}
				writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-Kirby-API-Key")
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func allowedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func allowedHeaders(value string) bool {
	for _, header := range strings.Split(value, ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "", "authorization", "content-type", "x-request-id", "x-kirby-api-key":
		default:
			return false
		}
	}
	return true
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			continue
		}
		switch character {
		case '.', '_', ':', '-':
		default:
			return false
		}
	}
	return true
}

type responseStatusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseStatusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseStatusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
