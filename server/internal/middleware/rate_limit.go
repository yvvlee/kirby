package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"

	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
)

type RateIdentity func(context.Context) string

// FixedWindowRateLimit uses the configured cache so limits remain effective across instances.
//
// Parameters: store holds shared counters; namespace separates policies; limit and window define the quota; identity selects its subject.
// Returns: rate-limiting middleware.
// Throws: returns unavailable on backend failure or too-many-requests when the quota is exhausted.
func FixedWindowRateLimit(store cache.Store, namespace string, limit int64, window time.Duration, identity RateIdentity) kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			if namespace == "" || limit <= 0 || window <= 0 {
				return nil, errorsv1.ErrorInternal("rate limiting is unavailable")
			}
			key := "rate:" + namespace + ":" + digestIdentity(identity(ctx))
			count, err := store.Increment(ctx, key, window)
			if err != nil {
				return nil, errorsv1.ErrorInternal("rate limiting is unavailable")
			}
			if count > limit {
				return nil, errorsv1.ErrorTooManyRequests("request rate limit exceeded")
			}
			return next(ctx, request)
		}
	}
}

// ClientIdentity derives the transport peer used by anonymous rate limits.
//
// Parameters: ctx contains the transport request.
// Returns: the peer host, or an unknown marker when transport metadata is absent.
// Throws: none.
func ClientIdentity(ctx context.Context) string {
	if request, ok := kratoshttp.RequestFromServerContext(ctx); ok {
		host, _, err := net.SplitHostPort(request.RemoteAddr)
		if err == nil {
			return host
		}
		return request.RemoteAddr
	}
	return "unknown"
}

// AuthorizationIdentity isolates authenticated rate limits by bearer identity.
//
// Parameters: ctx contains the transport request.
// Returns: the authorization value, falling back to the peer identity when absent.
// Throws: none.
func AuthorizationIdentity(ctx context.Context) string {
	if serverTransport, ok := transport.FromServerContext(ctx); ok {
		if value := serverTransport.RequestHeader().Get("Authorization"); value != "" {
			return value
		}
	}
	return ClientIdentity(ctx)
}

// RuntimeIdentity groups runtime requests by public API-key identifier.
//
// Parameters: ctx contains the transport request.
// Returns: the public key identifier, falling back to the peer identity when absent.
// Throws: none.
func RuntimeIdentity(ctx context.Context) string {
	if serverTransport, ok := transport.FromServerContext(ctx); ok {
		value := serverTransport.RequestHeader().Get("X-Kirby-API-Key")
		if value == "" {
			value = serverTransport.RequestHeader().Get("x-kirby-api-key")
		}
		if publicID, _, ok := strings.Cut(value, "."); ok && publicID != "" {
			return publicID
		}
	}
	return ClientIdentity(ctx)
}

func digestIdentity(value string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", len(value), value)))
	return hex.EncodeToString(digest[:])
}
