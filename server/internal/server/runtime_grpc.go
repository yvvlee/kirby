package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
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
		ctx = transport.NewServerContext(ctx, grpcTransport{operation: info.FullMethod, requestHeader: grpcHeader(incoming), replyHeader: grpcHeader(replyHeader)})
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
	return &runtimeGRPCServer{address: address, server: native, health: healthServer}
}

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

func (s *runtimeGRPCServer) ServiceInfo() map[string]grpc.ServiceInfo {
	return s.server.GetServiceInfo()
}

type grpcTransport struct {
	operation                  string
	requestHeader, replyHeader grpcHeader
}

func (grpcTransport) Kind() transport.Kind              { return transport.KindGRPC }
func (grpcTransport) Endpoint() string                  { return "" }
func (t grpcTransport) Operation() string               { return t.operation }
func (t grpcTransport) RequestHeader() transport.Header { return t.requestHeader }
func (t grpcTransport) ReplyHeader() transport.Header   { return t.replyHeader }

type grpcHeader metadata.MD

func (h grpcHeader) Get(key string) string {
	values := metadata.MD(h).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
func (h grpcHeader) Set(key, value string)      { metadata.MD(h).Set(key, value) }
func (h grpcHeader) Add(key, value string)      { metadata.MD(h).Append(key, value) }
func (h grpcHeader) Values(key string) []string { return metadata.MD(h).Get(key) }
func (h grpcHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
