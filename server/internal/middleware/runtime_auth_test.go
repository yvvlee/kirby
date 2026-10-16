package middleware

import (
	"context"
	"net/http"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	runtimev1 "github.com/yvvlee/kirby/server/api/runtime"
)

func TestRuntimeEntryPointsRequireExactlyOneCredential(t *testing.T) {
	request := &runtimev1.ConfigRequest{
		Project: "website",
		Key:     "feature",
	}

	ctx := transport.NewServerContext(context.Background(), middlewareTransport{header: header{}})
	_, err := HTTPAPIKey(func(context.Context, any) (any, error) {
		t.Fatal("missing credential must not reach the handler")
		return nil, nil
	})(ctx, request)
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
