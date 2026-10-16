package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/middleware"
)

type runtimeLogicFake struct {
	credentials []string
	result      *logic.RuntimeResult
	err         error
}

func (f *runtimeLogicFake) Read(_ context.Context, credential, _, _ string) (*logic.RuntimeResult, error) {
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
	fake := &runtimeLogicFake{result: &logic.RuntimeResult{Content: `{"enabled":true}`, Version: 7}}
	service := &RuntimeService{runtimeLogic: fake}
	request := &runtimev1.ConfigRequest{
		Project: "website",
		Key:     "feature",
	}

	header := testHeader{}
	header.Set(middleware.HTTPHeader, "http-secret")
	httpContext := transport.NewServerContext(context.Background(), testTransport{header: header})
	var httpReply *runtimev1.ConfigReply
	next := func(ctx context.Context, _ any) (any, error) {
		var err error
		httpReply, err = service.Config(ctx, request)
		return httpReply, err
	}
	_, err := middleware.HTTPAPIKey(kratosmiddleware.Handler(next))(httpContext, request)
	require.NoError(t, err)

	grpcContext := metadata.NewIncomingContext(context.Background(), metadata.Pairs(middleware.GRPCMetadata, "grpc-secret"))
	value, err := middleware.UnaryAPIKey(grpcContext, request, &grpc.UnaryServerInfo{FullMethod: runtimev1.Api_Config_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		return service.Config(ctx, request)
	})
	require.NoError(t, err)
	grpcReply := value.(*runtimev1.ConfigReply)

	assert.Equal(t, httpReply, grpcReply)
	assert.Equal(t, []string{"http-secret", "grpc-secret"}, fake.credentials)
}

func TestRuntimeServiceMapsAuthenticationAndScopeErrors(t *testing.T) {
	request := &runtimev1.ConfigRequest{
		Project: "website",
		Key:     "feature",
	}
	for _, test := range []struct {
		err  error
		code int32
	}{
		{
			err:  logic.ErrUnauthenticated,
			code: http.StatusUnauthorized,
		},
		{
			err:  logic.ErrProjectMismatch,
			code: http.StatusForbidden,
		},
	} {
		service := &RuntimeService{runtimeLogic: &runtimeLogicFake{err: test.err}}
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(middleware.GRPCMetadata, "secret"))
		_, err := middleware.UnaryAPIKey(ctx, request, nil, func(ctx context.Context, _ any) (any, error) {
			return service.Config(ctx, request)
		})
		assert.Equal(t, test.code, kratoserrors.FromError(err).Code)
	}
}

func TestRuntimeServiceLogsUnexpectedErrors(t *testing.T) {
	var output bytes.Buffer
	logger, err := infra.NewLoggerWithWriter(&output, conf.LogConfig{Level: "info"})
	require.NoError(t, err)
	service := &RuntimeService{
		runtimeLogic: &runtimeLogicFake{err: errors.New("database read failed")},
		logger:       logger,
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(middleware.GRPCMetadata, "secret"))

	_, err = middleware.UnaryAPIKey(ctx, nil, nil, func(ctx context.Context, _ any) (any, error) {
		return service.Config(ctx, &runtimev1.ConfigRequest{
			Project: "website",
			Key:     "feature",
		})
	})

	assert.Equal(t, int32(http.StatusInternalServerError), kratoserrors.FromError(err).Code)
	assert.Contains(t, output.String(), "database read failed")
}
