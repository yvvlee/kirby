package importer

import (
	"context"
	"net/http"
	"testing"
	"time"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/config"
	logic "github.com/yvvlee/kirby/server/internal/logic/importer"
	"github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type logicFake struct {
	request logic.Request
	actor   permission.Actor
	result  *logic.Result
}

func (f *logicFake) Import(_ context.Context, actor permission.Actor, request logic.Request) (*logic.Result, error) {
	f.actor, f.request = actor, request
	return f.result, nil
}

type userReader struct{ user *model.User }

func (r userReader) GetByID(context.Context, int64) (*model.User, error) { return r.user, nil }

type testTransport struct{ header testHeader }

func (testTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (testTransport) Endpoint() string                  { return "http://localhost" }
func (testTransport) Operation() string                 { return "test" }
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

func authenticatedContext(t *testing.T) context.Context {
	t.Helper()
	manager, err := authjwt.New(config.JWTConfig{Issuer: "test", ActiveKID: "primary", AccessTTL: config.Duration{Duration: 15 * time.Minute}, Keys: map[string]config.Secret{"primary": config.NewSecret("01234567890123456789012345678901")}})
	require.NoError(t, err)
	token, _, err := manager.Issue(9, "session")
	require.NoError(t, err)
	header := testHeader{}
	header.Set("Authorization", "Bearer "+token)
	ctx := transport.NewServerContext(context.Background(), testTransport{header: header})
	var result context.Context
	_, err = middleware.AdminAuth(manager, userReader{user: &model.User{Meta: model.Meta{ID: 9}, Enabled: true}})(kratosmiddleware.Handler(func(ctx context.Context, _ any) (any, error) { result = ctx; return nil, nil }))(ctx, nil)
	require.NoError(t, err)
	return result
}

func TestImportServiceMapsGeneratedContractToBusinessRequest(t *testing.T) {
	targetID := int64(50)
	fake := &logicFake{result: &logic.Result{Snapshot: &model.Snapshot{Meta: model.Meta{ID: 400}, ProjectID: 20, ConfigID: 50, ConfigKey: "feature", TagsJSON: "[]", Status: model.SnapshotStatusUnreleased}, Replayed: true}}
	service := &Service{logic: fake}
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
	_, err := (&Service{logic: &logicFake{}}).ImportSnapshot(context.Background(), &adminv1.ImportSnapshotRequest{TargetEnvironmentId: 2, SourceEnvironmentId: 1, SourceSnapshotId: 12, TargetProjectId: 20, Description: "Imported snapshot", IdempotencyKey: "request-00000001", ConflictStrategy: adminv1.ImportConflictStrategy_FAIL})
	assert.Error(t, err)
}
