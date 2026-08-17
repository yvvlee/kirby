package api_key

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"

	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

var apiKeyTestTime = time.Date(2026, time.August, 17, 8, 0, 0, 0, time.UTC)

type repositoryFake struct {
	item          *model.ProjectAPIKey
	createdDigest []byte
	findPublicID  string
}

func (r *repositoryFake) CreateTx(_ context.Context, _ *xorm.Session, _, projectID int64, item *model.ProjectAPIKey) error {
	item.ID = 41
	item.ProjectID = projectID
	r.createdDigest = append([]byte(nil), item.SecretHash...)
	clone := *item
	clone.SecretHash = append([]byte(nil), item.SecretHash...)
	r.item = &clone
	return nil
}

func (r *repositoryFake) List(context.Context, int64, int64) ([]model.ProjectAPIKey, error) {
	if r.item == nil {
		return nil, nil
	}
	return []model.ProjectAPIKey{*r.item}, nil
}

func (r *repositoryFake) FindByID(context.Context, int64, int64, int64) (*model.ProjectAPIKey, error) {
	if r.item == nil {
		return nil, errors.New("missing test key")
	}
	clone := *r.item
	r.findPublicID = clone.PublicID
	return &clone, nil
}

func (r *repositoryFake) LockByID(context.Context, *xorm.Session, int64, int64, int64) (*model.ProjectAPIKey, error) {
	clone := *r.item
	return &clone, nil
}

func (r *repositoryFake) RotateTx(_ context.Context, _ *xorm.Session, _, _, _ int64, currentPublicID string, replacement *model.ProjectAPIKey) error {
	if currentPublicID != r.item.PublicID {
		return errors.New("unexpected current public id")
	}
	clone := *replacement
	clone.SecretHash = append([]byte(nil), replacement.SecretHash...)
	r.item = &clone
	return nil
}

func (r *repositoryFake) RevokeTx(_ context.Context, _ *xorm.Session, _, _, _ int64, currentPublicID string, revokedAt time.Time) error {
	if currentPublicID != r.item.PublicID {
		return errors.New("unexpected current public id")
	}
	r.item.RevokedAt = &revokedAt
	return nil
}

type authorizerFake struct {
	allowed map[string]bool
	seen    []string
}

func (a *authorizerFake) Require(_ context.Context, _, _ int64, required ...string) error {
	a.seen = append(a.seen, required...)
	for _, key := range required {
		if !a.allowed[key] {
			return permission.ErrForbidden
		}
	}
	return nil
}

type auditFake struct{ actions []string }

func (a *auditFake) RecordForEnvironmentTx(_ context.Context, _ *xorm.Session, _ int64, item *model.AuditLog) error {
	a.actions = append(a.actions, item.Action)
	return nil
}

type transactorFake struct{}

func (transactorFake) WithTx(ctx context.Context, operation func(*xorm.Session) error) error {
	return operation(&xorm.Session{})
}

func newLogicForTest(t *testing.T, repository *repositoryFake, authorizer *authorizerFake, audits *auditFake) *Logic {
	t.Helper()
	manager, err := credential.New(config.NewSecret("01234567890123456789012345678901"))
	require.NoError(t, err)
	logicLayer, err := New(repository, manager, authorizer, audits, transactorFake{})
	require.NoError(t, err)
	logicLayer.now = func() time.Time { return apiKeyTestTime }
	return logicLayer
}

func TestCreateReturnsSecretOnceAndNeverReturnsDigest(t *testing.T) {
	repository := &repositoryFake{}
	authorizer := &authorizerFake{allowed: map[string]bool{permission.ProjectAPIKeyManage: true, permission.ProjectAPIKeyRead: true}}
	audits := &auditFake{}
	logicLayer := newLogicForTest(t, repository, authorizer, audits)

	created, err := logicLayer.Create(context.Background(), permission.Actor{UserID: 9, RequestID: "request"}, 5, 20, "production")
	require.NoError(t, err)
	assert.NotEmpty(t, created.Secret)
	assert.Nil(t, created.Key.SecretHash)
	assert.Len(t, repository.createdDigest, 32)
	assert.NotContains(t, string(repository.createdDigest), created.Secret)
	assert.Equal(t, []string{"project_api_key.create"}, audits.actions)

	listed, err := logicLayer.List(context.Background(), permission.Actor{UserID: 9}, 5, 20)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Nil(t, listed[0].SecretHash)
	assert.Equal(t, created.Key.SecretSuffix, listed[0].SecretSuffix)
}

func TestRotateInvalidatesOldPublicIDAndRevokeIsTerminal(t *testing.T) {
	repository := &repositoryFake{}
	authorizer := &authorizerFake{allowed: map[string]bool{permission.ProjectAPIKeyManage: true}}
	audits := &auditFake{}
	logicLayer := newLogicForTest(t, repository, authorizer, audits)
	created, err := logicLayer.Create(context.Background(), permission.Actor{UserID: 9}, 5, 20, "production")
	require.NoError(t, err)

	rotated, err := logicLayer.Rotate(context.Background(), permission.Actor{UserID: 9}, 5, 20, created.Key.ID)
	require.NoError(t, err)
	assert.NotEqual(t, created.Secret, rotated.Secret)
	assert.NotEqual(t, created.Key.PublicID, rotated.Key.PublicID)
	assert.Nil(t, rotated.Key.SecretHash)

	require.NoError(t, logicLayer.Revoke(context.Background(), permission.Actor{UserID: 9}, 5, 20, created.Key.ID))
	_, err = logicLayer.Rotate(context.Background(), permission.Actor{UserID: 9}, 5, 20, created.Key.ID)
	assert.ErrorIs(t, err, entity.ErrConflict)
	assert.Equal(t, []string{"project_api_key.create", "project_api_key.rotate", "project_api_key.revoke"}, audits.actions)
}

func TestManagementPermissionsAreDistinct(t *testing.T) {
	repository := &repositoryFake{}
	authorizer := &authorizerFake{allowed: map[string]bool{permission.ProjectAPIKeyRead: true}}
	logicLayer := newLogicForTest(t, repository, authorizer, &auditFake{})

	_, err := logicLayer.List(context.Background(), permission.Actor{UserID: 9}, 5, 20)
	require.NoError(t, err)
	_, err = logicLayer.Create(context.Background(), permission.Actor{UserID: 9}, 5, 20, "denied")
	assert.ErrorIs(t, err, permission.ErrForbidden)
}
