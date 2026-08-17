package role

import (
	"context"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
)

func TestCreateRoleRejectsInvalidRequestBeforeBusinessLogic(t *testing.T) {
	service := &Service{}
	_, err := service.CreateRole(context.Background(), nil)
	if err == nil || kratoserrors.FromError(err).Code != 400 {
		t.Fatalf("invalid request error = %v", err)
	}
}
