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

	errorsv1 "github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	"github.com/yvvlee/kirby/server/internal/storage/cache"
)

type RateIdentity func(context.Context) string

// FixedWindowRateLimit uses the configured cache so limits remain effective across instances.
func FixedWindowRateLimit(store cache.Store, namespace string, limit int64, window time.Duration, identity RateIdentity) kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			if store == nil || namespace == "" || limit <= 0 || window <= 0 || identity == nil {
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

func AuthorizationIdentity(ctx context.Context) string {
	if serverTransport, ok := transport.FromServerContext(ctx); ok {
		if value := serverTransport.RequestHeader().Get("Authorization"); value != "" {
			return value
		}
	}
	return ClientIdentity(ctx)
}

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
