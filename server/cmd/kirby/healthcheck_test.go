package kirby

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

func TestRunHealthcheckChecksHTTPAndGRPC(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(httpServer.Close)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	err = runHealthcheck(context.Background(), healthcheckOptions{
		httpEndpoint: httpServer.URL,
		grpcAddress:  listener.Addr().String(),
		timeout:      time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunHealthcheckFailsOnUnreadyHTTP(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(httpServer.Close)

	err := runHealthcheck(context.Background(), healthcheckOptions{
		httpEndpoint: httpServer.URL,
		grpcAddress:  "127.0.0.1:1",
		timeout:      time.Second,
	})
	if err == nil {
		t.Fatal("unready HTTP endpoint was accepted")
	}
}
