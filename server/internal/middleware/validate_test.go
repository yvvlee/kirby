package middleware

import (
	"context"
	"errors"
	"net/http"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
)

func TestValidateRequestRejectsGeneratedPGVFailureBeforeHandler(t *testing.T) {
	called := false
	handler := ValidateRequest()(func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})

	_, err := handler(context.Background(), &adminv1.LoginRequest{
		Username: "",
		Password: "password",
	})

	require.False(t, called)
	failure := kratoserrors.FromError(err)
	require.Equal(t, int32(http.StatusBadRequest), failure.Code)
	require.Equal(t, "VALIDATOR", failure.Reason)
	require.NotNil(t, errors.Unwrap(failure))
}

func TestValidateRequestPreservesValidRequestsAndHandlerErrors(t *testing.T) {
	request := &adminv1.LoginRequest{
		Username: "admin",
		Password: "password",
	}
	handlerErr := errors.New("business failure")
	handler := ValidateRequest()(func(_ context.Context, actual any) (any, error) {
		require.Same(t, request, actual)
		return nil, handlerErr
	})

	_, err := handler(context.Background(), request)

	require.ErrorIs(t, err, handlerErr)
}
