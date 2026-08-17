package api_key

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	logic "github.com/yvvlee/kirby/server/internal/logic/api_key"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type serviceLogicFake struct {
	items  []model.ProjectAPIKey
	secret *logic.SecretResult
}

func (f serviceLogicFake) List(context.Context, permission.Actor, int64, int64) ([]model.ProjectAPIKey, error) {
	return f.items, nil
}
func (f serviceLogicFake) Create(context.Context, permission.Actor, int64, int64, string) (*logic.SecretResult, error) {
	return f.secret, nil
}
func (f serviceLogicFake) Rotate(context.Context, permission.Actor, int64, int64, int64) (*logic.SecretResult, error) {
	return f.secret, nil
}
func (serviceLogicFake) Revoke(context.Context, permission.Actor, int64, int64, int64) error {
	return nil
}

func TestProjectAPIKeyProtoNeverContainsDigest(t *testing.T) {
	now := time.Date(2026, time.August, 17, 8, 0, 0, 0, time.UTC)
	item := model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{ID: 41, CreatedAt: now, UpdatedAt: now},
		ProjectID:  20, PublicID: "kirby_pk_public", Name: "production",
		SecretHash: []byte("database-only-digest"), SecretSuffix: "abcd", CreatedBy: 9,
	}
	converted := toProto(&item)
	assert.Equal(t, int64(41), converted.Id)
	assert.Equal(t, "abcd", converted.SecretSuffix)
	assert.NotContains(t, converted.String(), "database-only-digest")
}

func TestProjectAPIKeyServiceRejectsRequestsWithoutAdminIdentity(t *testing.T) {
	service := &Service{logic: serviceLogicFake{}}
	_, err := service.ListProjectApiKeys(context.Background(), &adminv1.ProjectApiKeyScopeRequest{EnvironmentId: 5, ProjectId: 20})
	assert.Error(t, err)
	_, err = service.CreateProjectApiKey(context.Background(), nil)
	assert.Error(t, err)
}

func TestSecretReplyFailsOnIncompleteLogicResult(t *testing.T) {
	_, err := secretReply(nil, nil)
	require.Error(t, err)
}
