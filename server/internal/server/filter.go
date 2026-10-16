package server

import (
	"context"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/infra"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

type apiRequestContextKey struct{}

// APIPrefix preserves the browser-facing /api namespace while generated
// handlers continue to use their protobuf-defined paths.
//
// Parameters: none.
// Returns: the HTTP route-rewriting filter.
// Throws: returns HTTP 404 for reserved management paths outside the public API prefix.
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
	for _, prefix := range []string{"/admin", "/auth", "/setup/status", "/setup/admin"} {
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

// HTTPBoundary records request outcomes and guards the HTTP transport boundary.
//
// Parameters: logger is the process logger.
// Returns: the HTTP boundary filter.
// Throws: writes an internal-server-error response when a handler panics.
func HTTPBoundary(logger *infra.Logger) kratoshttp.FilterFunc {
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
			statusWriter := &responseStatusWriter{
				ResponseWriter: writer,
				status:         http.StatusOK,
			}
			started := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Errorw("msg", "HTTP handler panic", "request_id", requestID, "method", request.Method, "path", request.URL.Path, "stack", string(debug.Stack()))
					if !statusWriter.wroteHeader {
						http.Error(statusWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
				logger.Infow("msg", "HTTP request", "request_id", requestID, "method", request.Method, "path", request.URL.Path, "status", statusWriter.status, "duration", time.Since(started))
			}()
			next.ServeHTTP(statusWriter, request)
		})
	}
}

// SameOrigin rejects browser requests from a different public origin.
// Parameters: origins resolves the public request address through trusted proxies.
// Returns: an HTTP filter that keeps non-browser clients usable without CORS.
// Throws: no errors; invalid browser origins receive HTTP 403.
func SameOrigin(origins *authsession.OriginValidator) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if len(request.Header.Values("Origin")) == 0 {
				next.ServeHTTP(writer, request)
				return
			}
			if err := origins.Validate(request); err != nil {
				http.Error(writer, "request origin is not allowed", http.StatusForbidden)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
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

// WriteHeader records the first response status for request logging.
//
// Parameters: status is the HTTP status code.
// Returns: none.
// Throws: none.
func (w *responseStatusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

// Write records an implicit success status before forwarding response bytes.
//
// Parameters: body is the response payload.
// Returns: the byte count written.
// Throws: the underlying response writer error.
func (w *responseStatusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
