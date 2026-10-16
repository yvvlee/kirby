package middleware

import (
	"context"
	"time"

	"github.com/yvvlee/kirby/server/internal/infra"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
)

// TransportLog assigns a request ID and logs only transport metadata.
//
// Parameters: logger is the process logger.
// Returns: middleware that logs duration, operation, and status.
// Throws: propagates downstream handler errors.
func TransportLog(logger *infra.Logger) kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, request any) (reply any, err error) {
			serverTransport, ok := transport.FromServerContext(ctx)
			if !ok {
				return next(ctx, request)
			}
			requestID := serverTransport.RequestHeader().Get("X-Request-ID")
			if !safeRequestID(requestID) {
				requestID = uuid.NewString()
			}
			serverTransport.RequestHeader().Set("X-Request-ID", requestID)
			serverTransport.ReplyHeader().Set("X-Request-ID", requestID)
			started := time.Now()
			reply, err = next(ctx, request)
			logger.WithContext(ctx).Infow("msg", "transport request", "request_id", requestID, "kind", serverTransport.Kind().String(), "operation", serverTransport.Operation(), "duration", time.Since(started), "error", err)
			return reply, err
		}
	}
}

func safeRequestID(value string) bool {
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
