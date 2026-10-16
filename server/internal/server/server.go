// Package server exposes Kirby's management HTTP and runtime HTTP/gRPC transports.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2"

	"github.com/yvvlee/kirby/server/internal/health"
	"github.com/yvvlee/kirby/server/internal/version"
)

const (
	adminRateLimit   = 600
	authRateLimit    = 60
	runtimeRateLimit = 1200
	rateWindow       = time.Minute
)

// NewApplication registers every public transport before any listener starts.
//
// Parameters: ctx controls construction; deps supplies shared services; webHandler serves the optional browser app.
// Returns: the configured Kratos application.
// Throws: an error when a transport cannot be configured.
func NewApplication(ctx context.Context, deps *Dependencies, webHandler http.Handler) (*kratos.App, error) {
	probe := health.NewProbe()
	httpServer, err := NewHTTPServer(deps, webHandler, probe)
	if err != nil {
		return nil, err
	}
	grpcServer := NewGRPCServer(deps)
	return kratos.New(
		kratos.Name("kirby"), kratos.Version(version.Version), kratos.Context(ctx),
		kratos.Server(httpServer, grpcServer),
		kratos.Logger(deps.Logger),
		kratos.AfterStart(func(context.Context) error { probe.SetReady(true); return nil }),
		kratos.BeforeStop(func(context.Context) error { probe.SetReady(false); return nil }),
	), nil
}
