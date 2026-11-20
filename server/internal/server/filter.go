package server

import (
	"context"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/i18n"
	"github.com/yvvlee/kirby/server/internal/infra"
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
					localizedErrorEncoder(writer, request, errorsv1.ErrorNotFound("resource not found"))
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
					logger.Errorw("msg", "HTTP handler panic", "request_id", requestID, "method", request.Method, "path", logRequestPath(request.URL.Path), "stack", string(debug.Stack()))
					if !statusWriter.wroteHeader {
						localizedErrorEncoder(statusWriter, request, errorsv1.ErrorInternal("operation failed"))
					}
				}
				logger.Infow("msg", "HTTP request", "request_id", requestID, "method", request.Method, "path", logRequestPath(request.URL.Path), "status", statusWriter.status, "duration", time.Since(started))
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
				localizedErrorEncoder(writer, request, errorsv1.ErrorForbidden("request origin is not allowed"))
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

// Browser invitation URLs share their path with the generated API routes. Route
// only unprefixed browser reads to the SPA before the API router sees them.
func browserInvitations(web http.Handler) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if !strings.HasPrefix(request.URL.Path, "/invitations/") {
				next.ServeHTTP(writer, request)
				return
			}
			if web == nil || (request.Method != http.MethodGet && request.Method != http.MethodHead) {
				writeLocalizedHTTPError(writer, request, http.StatusNotFound, "resource not found")
				return
			}
			web.ServeHTTP(writer, request)
		})
	}
}

func writeLocalizedHTTPError(writer http.ResponseWriter, request *http.Request, status int, message string) {
	locale := i18n.Negotiate(request.Header.Get("Accept-Language"))
	writer.Header().Del("Content-Length")
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(writer, i18n.Translate(locale, message), status)
}

type localizedHTTPResponseWriter struct {
	http.ResponseWriter
	request *http.Request
	failed  bool
}

// WriteHeader localizes errors emitted by the standard library file handlers.
// Parameters: status is the response status from the wrapped handler.
// Returns: none.
// Throws: none; the response is written to the underlying writer.
func (w *localizedHTTPResponseWriter) WriteHeader(status int) {
	if status >= http.StatusBadRequest {
		w.failed = true
		writeLocalizedHTTPError(w.ResponseWriter, w.request, status, http.StatusText(status))
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write forwards file bytes and discards the standard library's English error body.
// Parameters: value contains response bytes.
// Returns: the number of bytes handled and any underlying write error.
// Throws: none; write failures are returned.
func (w *localizedHTTPResponseWriter) Write(value []byte) (int, error) {
	if w.failed {
		return len(value), nil
	}
	return w.ResponseWriter.Write(value)
}

func logRequestPath(path string) string {
	for _, prefix := range []string{"/api/invitations/", "/invitations/"} {
		if strings.HasPrefix(path, prefix) {
			return prefix + "[redacted]"
		}
	}
	return path
}
