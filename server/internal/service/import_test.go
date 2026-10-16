package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type importLogicFake struct {
	request logic.ImportRequest
	actor   permission.Actor
	result  *logic.ImportResult
}

func (f *importLogicFake) Import(_ context.Context, actor permission.Actor, request logic.ImportRequest) (*logic.ImportResult, error) {
	f.actor, f.request = actor, request
	return f.result, nil
}

type userReader struct{ user *model.User }

func (r userReader) GetByID(context.Context, int64) (*model.User, error) { return r.user, nil }

type importTestTransport struct{ header importTestHeader }

func (importTestTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (importTestTransport) Endpoint() string                  { return "http://localhost" }
func (importTestTransport) Operation() string                 { return "test" }
func (t importTestTransport) RequestHeader() transport.Header { return t.header }
func (importTestTransport) ReplyHeader() transport.Header     { return importTestHeader{} }

type importTestHeader http.Header

func (h importTestHeader) Get(key string) string      { return http.Header(h).Get(key) }
func (h importTestHeader) Set(key, value string)      { http.Header(h).Set(key, value) }
func (h importTestHeader) Add(key, value string)      { http.Header(h).Add(key, value) }
func (h importTestHeader) Values(key string) []string { return http.Header(h).Values(key) }
func (h importTestHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}

func authenticatedContext(t *testing.T) context.Context {
	t.Helper()
	manager, err := authjwt.New(authjwt.Config{
		Issuer:    "test",
		ActiveKID: "primary",
		AccessTTL: 15 * time.Minute,
		Keys: map[string]conf.Secret{
			"primary": conf.NewSecret("01234567890123456789012345678901"),
		},
	})
	require.NoError(t, err)
	token, _, err := manager.Issue(9, "session")
	require.NoError(t, err)
	header := importTestHeader{}
	header.Set("Authorization", "Bearer "+token)
	ctx := transport.NewServerContext(context.Background(), importTestTransport{header: header})
	var result context.Context
	_, err = middleware.AdminAuth(manager, userReader{user: &model.User{
		Meta:    model.Meta{ID: 9},
		Enabled: true,
	}})(kratosmiddleware.Handler(func(ctx context.Context, _ any) (any, error) { result = ctx; return nil, nil }))(ctx, nil)
	require.NoError(t, err)
	return result
}

func TestImportServiceMapsGeneratedContractToBusinessRequest(t *testing.T) {
	targetID := int64(50)
	fake := &importLogicFake{result: &logic.ImportResult{Snapshot: &model.Snapshot{Meta: model.Meta{ID: 400}, ProjectID: 20, ConfigID: 50, ConfigKey: "feature", TagsJSON: "[]", Status: model.SnapshotStatusUnreleased}, Replayed: true}}
	service := &ImportService{importLogic: fake}
	reply, err := service.ImportSnapshot(authenticatedContext(t), &adminv1.ImportSnapshotRequest{
		TargetEnvironmentId: 2, SourceEnvironmentId: 1, SourceSnapshotId: 12, TargetProjectId: 20,
		TargetConfigId: &targetID, Description: "Imported snapshot", Tags: []commonv1.Snapshot_Tag{commonv1.Snapshot_REUSE},
		IdempotencyKey: "request-00000001", ConflictStrategy: adminv1.ImportConflictStrategy_REPLACE,
	})
	require.NoError(t, err)
	assert.True(t, reply.Replayed)
	assert.Equal(t, int64(9), fake.actor.UserID)
	assert.Equal(t, logic.StrategyReplace, fake.request.ConflictStrategy)
	assert.Equal(t, &targetID, fake.request.TargetConfigID)
}

func TestImportServiceRejectsMissingCurrentJWTIdentity(t *testing.T) {
	_, err := (&ImportService{importLogic: &importLogicFake{}}).ImportSnapshot(context.Background(), &adminv1.ImportSnapshotRequest{
		TargetEnvironmentId: 2,
		SourceEnvironmentId: 1,
		SourceSnapshotId:    12,
		TargetProjectId:     20,
		Description:         "Imported snapshot",
		IdempotencyKey:      "request-00000001",
		ConflictStrategy:    adminv1.ImportConflictStrategy_FAIL,
	})
	assert.Error(t, err)
}
