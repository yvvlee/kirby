package service

import (
	"context"
	"net/http"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
)

func TestCreateEnvironmentRequiresAuthentication(t *testing.T) {
	service := &EnvironmentService{}
	_, err := service.CreateEnvironment(context.Background(), &adminv1.CreateEnvironmentRequest{
		ProjectId: 1,
		Key:       "prod",
		Name:      "Production",
	})
	if err == nil {
		t.Fatal("expected error without actor context")
	}
}

type scopedEnvironmentLogic struct {
	environmentLogic
	verifiedProject int64
	verifyErr       error
	updates         int
}

func (l *scopedEnvironmentLogic) VerifyProject(_ context.Context, _, projectID int64) error {
	l.verifiedProject = projectID
	return l.verifyErr
}

func (l *scopedEnvironmentLogic) Update(context.Context, permission.Actor, int64, repository.EnvironmentUpdate) (*model.Environment, error) {
	l.updates++
	return nil, nil
}

type environmentTestVerifier struct{}

func (environmentTestVerifier) Verify(string) (*authjwt.Claims, error) {
	return &authjwt.Claims{
		RegisteredClaims: jwtlib.RegisteredClaims{Subject: "9"},
	}, nil
}

type environmentTestUsers struct{}

func (environmentTestUsers) GetByID(context.Context, int64) (*model.User, error) {
	return &model.User{
		Meta:    model.Meta{ID: 9},
		Enabled: true,
	}, nil
}

func TestUpdateEnvironmentRejectsForeignProjectBeforeMutation(t *testing.T) {
	layer := &scopedEnvironmentLogic{verifyErr: repository.Missing("environment")}
	service := &EnvironmentService{environmentLogic: layer}
	ctx, tr := requestContext(http.MethodPut, "https://kirby.example.com", "")
	tr.request.Header.Set("Authorization", "Bearer test-token")
	request := &adminv1.UpdateEnvironmentRequest{
		ProjectId:     40,
		EnvironmentId: 2,
	}
	handler := middleware.AdminAuth(environmentTestVerifier{}, environmentTestUsers{})(func(ctx context.Context, _ any) (any, error) {
		return service.UpdateEnvironment(ctx, request)
	})
	_, err := handler(ctx, request)
	require.Equal(t, http.StatusNotFound, kratoserrors.Code(err))
	require.Equal(t, int64(40), layer.verifiedProject)
	require.Zero(t, layer.updates)
}
