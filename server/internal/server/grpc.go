package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
)

type runtimeGRPCServer struct {
	address  string
	server   *grpc.Server
	health   *health.Server
	mu       sync.Mutex
	listener net.Listener
}

func newRuntimeGRPCServer(address string, timeout time.Duration, middlewares []kratosmiddleware.Middleware, interceptors ...grpc.UnaryServerInterceptor) *runtimeGRPCServer {
	boundary := func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		incoming, _ := metadata.FromIncomingContext(ctx)
		replyHeader := metadata.MD{}
		ctx = transport.NewServerContext(ctx, grpcTransport{
			operation:     info.FullMethod,
			requestHeader: grpcHeader(incoming),
			replyHeader:   grpcHeader(replyHeader),
		})
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		handlerWithMiddleware := kratosmiddleware.Chain(middlewares...)(func(ctx context.Context, request any) (any, error) {
			return handler(ctx, request)
		})
		reply, err := handlerWithMiddleware(ctx, request)
		if len(replyHeader) > 0 {
			_ = grpc.SetHeader(ctx, replyHeader)
		}
		return reply, err
	}
	chain := append([]grpc.UnaryServerInterceptor{boundary}, interceptors...)
	native := grpc.NewServer(grpc.ChainUnaryInterceptor(chain...))
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(native, healthServer)
	reflection.Register(native)
	return &runtimeGRPCServer{
		address: address,
		server:  native,
		health:  healthServer,
	}
}

// Start opens the runtime listener and advertises service readiness.
//
// Parameters: the context is supplied by the application lifecycle.
// Returns: nil when serving stops normally.
// Throws: a listener or gRPC serving error.
func (s *runtimeGRPCServer) Start(context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()
	s.health.Resume()
	err = s.server.Serve(listener)
	if errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}

// Stop allows active RPCs to finish within the shutdown deadline.
//
// Parameters: ctx bounds graceful shutdown.
// Returns: nil after shutdown completes.
// Throws: the context error if shutdown exceeds its deadline.
func (s *runtimeGRPCServer) Stop(ctx context.Context) error {
	s.health.Shutdown()
	done := make(chan struct{})
	go func() { s.server.GracefulStop(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.server.Stop()
		return ctx.Err()
	}
}

// ServiceInfo exposes registered services for readiness and wiring checks.
//
// Parameters: none.
// Returns: the gRPC service registry.
// Throws: none.
func (s *runtimeGRPCServer) ServiceInfo() map[string]grpc.ServiceInfo {
	return s.server.GetServiceInfo()
}

type grpcTransport struct {
	operation     string
	requestHeader grpcHeader
	replyHeader   grpcHeader
}

// Kind identifies this adapter as the gRPC transport.
//
// Parameters: none.
// Returns: transport.KindGRPC.
// Throws: none.
func (grpcTransport) Kind() transport.Kind { return transport.KindGRPC }

// Endpoint avoids claiming a public endpoint for the native gRPC adapter.
//
// Parameters: none.
// Returns: an empty endpoint string.
// Throws: none.
func (grpcTransport) Endpoint() string { return "" }

// Operation exposes the full method name used by middleware selectors.
//
// Parameters: none.
// Returns: the current gRPC method.
// Throws: none.
func (t grpcTransport) Operation() string { return t.operation }

// RequestHeader exposes incoming metadata through the Kratos contract.
//
// Parameters: none.
// Returns: the incoming metadata adapter.
// Throws: none.
func (t grpcTransport) RequestHeader() transport.Header { return t.requestHeader }

// ReplyHeader collects metadata to attach to the gRPC response.
//
// Parameters: none.
// Returns: the outgoing metadata adapter.
// Throws: none.
func (t grpcTransport) ReplyHeader() transport.Header { return t.replyHeader }

type grpcHeader metadata.MD

// Get implements first-value header lookup for transport middleware.
//
// Parameters: key is the metadata name.
// Returns: its first value, or an empty string.
// Throws: none.
func (h grpcHeader) Get(key string) string {
	values := metadata.MD(h).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set replaces metadata values for one header name.
//
// Parameters: key identifies the header; value is its replacement.
// Returns: none.
// Throws: none.
func (h grpcHeader) Set(key, value string) { metadata.MD(h).Set(key, value) }

// Add preserves existing values when appending response metadata.
//
// Parameters: key identifies the header; value is appended.
// Returns: none.
// Throws: none.
func (h grpcHeader) Add(key, value string) { metadata.MD(h).Append(key, value) }

// Values exposes all values for duplicate-header validation.
//
// Parameters: key identifies the header.
// Returns: the metadata values for that name.
// Throws: none.
func (h grpcHeader) Values(key string) []string { return metadata.MD(h).Get(key) }

// Keys exposes metadata names through the Kratos header contract.
//
// Parameters: none.
// Returns: the present header names.
// Throws: none.
func (h grpcHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}

// NewGRPCServer registers the runtime service and its authentication boundary.
//
// Parameters: deps supplies configuration, services, storage, and logging.
// Returns: the configured runtime gRPC server.
// Throws: none.
func NewGRPCServer(deps *Dependencies) *runtimeGRPCServer {
	runtimeRate := adminmiddleware.FixedWindowRateLimit(deps.Store.Cache, "runtime", runtimeRateLimit, rateWindow, adminmiddleware.RuntimeIdentity)
	runtimeGRPCSelector := selector.Server(runtimeRate).Path(runtimev1.OperationApiConfig).Build()
	grpcServer := newRuntimeGRPCServer(
		deps.Config.GRPC.Address,
		deps.Config.GRPC.Timeout.Duration,
		[]kratosmiddleware.Middleware{recovery.Recovery(), adminmiddleware.ValidateRequest(), adminmiddleware.TransportLog(deps.Logger), runtimeGRPCSelector},
		adminmiddleware.UnaryAPIKey,
	)
	registerGRPC(grpcServer, deps.Runtime)

	return grpcServer
}

func registerGRPC(server *runtimeGRPCServer, runtimeService runtimev1.ApiServer) {
	runtimev1.RegisterApiServer(server.server, runtimeService)
}
