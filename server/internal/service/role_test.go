package service

import (
	"context"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
)

func TestCreateRoleRejectsMissingActor(t *testing.T) {
	service := &RoleService{}
	_, err := service.CreateRole(context.Background(), nil)
	if err == nil || kratoserrors.FromError(err).Code != 403 {
		t.Fatalf("expected 403 forbidden, got %v", err)
	}
}
