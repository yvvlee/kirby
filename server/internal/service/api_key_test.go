package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type serviceLogicFake struct {
	items  []*commonv1.ProjectApiKey
	secret *logic.APIKeySecretResult
}

func (f serviceLogicFake) List(context.Context, permission.Actor, int64, int64) ([]*commonv1.ProjectApiKey, error) {
	return f.items, nil
}
func (f serviceLogicFake) Create(context.Context, *entity.CreateAPIKeyParam) (*logic.APIKeySecretResult, error) {
	return f.secret, nil
}
func (f serviceLogicFake) Rotate(context.Context, *entity.RotateAPIKeyParam) (*logic.APIKeySecretResult, error) {
	return f.secret, nil
}
func (serviceLogicFake) Revoke(context.Context, *entity.RevokeAPIKeyParam) error {
	return nil
}

func TestProjectAPIKeyServiceRejectsRequestsWithoutAdminIdentity(t *testing.T) {
	service := &APIKeyService{apiKeyLogic: serviceLogicFake{}}
	_, err := service.ListProjectApiKeys(context.Background(), &adminv1.ProjectApiKeyScopeRequest{
		EnvironmentId: 5,
		ProjectId:     20,
	})
	assert.Error(t, err)
	_, err = service.CreateProjectApiKey(context.Background(), &adminv1.CreateProjectApiKeyRequest{
		EnvironmentId: 5,
		ProjectId:     20,
		Name:          "key",
	})
	assert.Error(t, err)
}
