package runtime

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	runtimev1 "github.com/yvvlee/kirby/server/gen/kirby/runtime/v1"
	logic "github.com/yvvlee/kirby/server/internal/logic/runtime"
)

type logicFake struct {
	credentials []string
	result      *logic.Result
	err         error
}

func (f *logicFake) Read(_ context.Context, credential, _, _ string) (*logic.Result, error) {
	f.credentials = append(f.credentials, credential)
	return f.result, f.err
}

type testTransport struct{ header testHeader }

func (testTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (testTransport) Endpoint() string                  { return "http://localhost" }
func (testTransport) Operation() string                 { return runtimev1.OperationApiConfig }
func (t testTransport) RequestHeader() transport.Header { return t.header }
func (testTransport) ReplyHeader() transport.Header     { return testHeader{} }

type testHeader http.Header

func (h testHeader) Get(key string) string      { return http.Header(h).Get(key) }
func (h testHeader) Set(key, value string)      { http.Header(h).Set(key, value) }
func (h testHeader) Add(key, value string)      { http.Header(h).Add(key, value) }
func (h testHeader) Values(key string) []string { return http.Header(h).Values(key) }
func (h testHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}

func TestHTTPAndGRPCUseSameRuntimeService(t *testing.T) {
	fake := &logicFake{result: &logic.Result{Content: `{"enabled":true}`, Version: 7}}
	service := &Service{logic: fake}
	request := &runtimev1.ConfigRequest{Project: "website", Key: "feature"}

	header := testHeader{}
	header.Set(HTTPHeader, "http-secret")
	httpContext := transport.NewServerContext(context.Background(), testTransport{header: header})
	var httpReply *runtimev1.ConfigReply
	next := func(ctx context.Context, _ any) (any, error) {
		var err error
		httpReply, err = service.Config(ctx, request)
		return httpReply, err
	}
	_, err := HTTPAPIKey(kratosmiddleware.Handler(next))(httpContext, request)
	require.NoError(t, err)

	grpcContext := metadata.NewIncomingContext(context.Background(), metadata.Pairs(GRPCMetadata, "grpc-secret"))
	value, err := UnaryAPIKey(grpcContext, request, &grpc.UnaryServerInfo{FullMethod: runtimev1.Api_Config_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		return service.Config(ctx, request)
	})
	require.NoError(t, err)
	grpcReply := value.(*runtimev1.ConfigReply)

	assert.Equal(t, httpReply, grpcReply)
	assert.Equal(t, []string{"http-secret", "grpc-secret"}, fake.credentials)
}

func TestRuntimeEntryPointsRequireExactlyOneCredential(t *testing.T) {
	service := &Service{logic: &logicFake{result: &logic.Result{}}}
	request := &runtimev1.ConfigRequest{Project: "website", Key: "feature"}

	ctx := transport.NewServerContext(context.Background(), testTransport{header: testHeader{}})
	_, err := HTTPAPIKey(func(ctx context.Context, _ any) (any, error) { return service.Config(ctx, request) })(ctx, request)
	assert.Equal(t, int32(http.StatusUnauthorized), kratoserrors.FromError(err).Code)

	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(GRPCMetadata, "one", GRPCMetadata, "two"))
	_, err = UnaryAPIKey(ctx, request, nil, func(context.Context, any) (any, error) { return nil, nil })
	assert.Equal(t, int32(http.StatusUnauthorized), kratoserrors.FromError(err).Code)
}

func TestGRPCAPIKeyInterceptorLeavesHealthServicePublic(t *testing.T) {
	called := false
	result, err := UnaryAPIKey(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}, func(context.Context, any) (any, error) {
		called = true
		return "serving", nil
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "serving", result)
}

func TestRuntimeServiceMapsAuthenticationAndScopeErrors(t *testing.T) {
	request := &runtimev1.ConfigRequest{Project: "website", Key: "feature"}
	for _, test := range []struct {
		err  error
		code int32
	}{
		{err: logic.ErrUnauthenticated, code: http.StatusUnauthorized},
		{err: logic.ErrProjectMismatch, code: http.StatusForbidden},
	} {
		service := &Service{logic: &logicFake{err: test.err}}
		ctx := context.WithValue(context.Background(), credentialKey{}, "secret")
		_, err := service.Config(ctx, request)
		assert.Equal(t, test.code, kratoserrors.FromError(err).Code)
	}
}

func TestRuntimeServiceLogsUnexpectedErrors(t *testing.T) {
	var output bytes.Buffer
	service := &Service{
		logic:  &logicFake{err: errors.New("database read failed")},
		logger: slog.New(slog.NewTextHandler(&output, nil)),
	}
	ctx := context.WithValue(context.Background(), credentialKey{}, "secret")

	_, err := service.Config(ctx, &runtimev1.ConfigRequest{Project: "website", Key: "feature"})

	assert.Equal(t, int32(http.StatusInternalServerError), kratoserrors.FromError(err).Code)
	assert.Contains(t, output.String(), "database read failed")
}
