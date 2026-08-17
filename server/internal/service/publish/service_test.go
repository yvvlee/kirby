package publish

import (
	"context"
	"net/http"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type serviceLogic struct {
	result      *model.Snapshot
	err         error
	actor       permission.Actor
	environment int64
	snapshot    int64
	version     uint32
	action      string
}

func (l *serviceLogic) Publish(_ context.Context, actor permission.Actor, environmentID, snapshotID int64, version uint32) (*model.Snapshot, error) {
	l.actor, l.environment, l.snapshot, l.version, l.action = actor, environmentID, snapshotID, version, "publish"
	return l.result, l.err
}

func (l *serviceLogic) Unpublish(_ context.Context, actor permission.Actor, environmentID, snapshotID int64, version uint32) (*model.Snapshot, error) {
	l.actor, l.environment, l.snapshot, l.version, l.action = actor, environmentID, snapshotID, version, "unpublish"
	return l.result, l.err
}

func serviceSnapshot(status model.SnapshotStatus) *model.Snapshot {
	now := time.Date(2026, time.August, 17, 6, 7, 8, 0, time.UTC)
	return &model.Snapshot{
		Meta:      model.Meta{ID: 12, Version: 4, CreatedAt: now, UpdatedAt: now},
		ProjectID: 3, ConfigID: 4, ConfigKey: "feature", Description: "snapshot",
		Content: `{}`, TagsJSON: `[]`, Status: status,
	}
}

func TestPublicationServiceRejectsInvalidRequestBeforeAuthentication(t *testing.T) {
	service := &Service{}
	_, err := service.PublishSnapshot(context.Background(), nil)
	require.Error(t, err)
	assert.Equal(t, int32(http.StatusBadRequest), kratoserrors.FromError(err).Code)
}

func TestPublicationServiceRequiresAuthenticatedActor(t *testing.T) {
	logicLayer := &serviceLogic{}
	service := &Service{logic: logicLayer}
	_, err := service.PublishSnapshot(context.Background(), &adminv1.PublishSnapshotRequest{EnvironmentId: 5, SnapshotId: 12})
	require.Error(t, err)
	assert.Equal(t, int32(http.StatusForbidden), kratoserrors.FromError(err).Code)
	assert.Empty(t, logicLayer.action)
}

func TestPublicationServicePassesActorScopeAndVersion(t *testing.T) {
	logicLayer := &serviceLogic{result: serviceSnapshot(model.SnapshotStatusReleased)}
	service := &Service{logic: logicLayer}
	ctx := authenticatedServiceContext(t, 7)

	reply, err := service.PublishSnapshot(ctx, &adminv1.PublishSnapshotRequest{EnvironmentId: 5, SnapshotId: 12, Version: 3})

	require.NoError(t, err)
	assert.Equal(t, int64(12), reply.Snapshot.Id)
	assert.Equal(t, "publish", logicLayer.action)
	assert.Equal(t, int64(7), logicLayer.actor.UserID)
	assert.Equal(t, int64(5), logicLayer.environment)
	assert.Equal(t, int64(12), logicLayer.snapshot)
	assert.Equal(t, uint32(3), logicLayer.version)

	logicLayer.result = serviceSnapshot(model.SnapshotStatusUnreleased)
	reply, err = service.UnpublishSnapshot(ctx, &adminv1.PublishSnapshotRequest{EnvironmentId: 5, SnapshotId: 12, Version: 4})
	require.NoError(t, err)
	assert.Equal(t, int64(12), reply.Snapshot.Id)
	assert.Equal(t, "unpublish", logicLayer.action)
	assert.Equal(t, uint32(4), logicLayer.version)
}

func TestPublicationServiceMapsVersionConflict(t *testing.T) {
	logicLayer := &serviceLogic{err: entity.Conflict("stale")}
	service := &Service{logic: logicLayer}
	_, err := service.PublishSnapshot(authenticatedServiceContext(t, 7), &adminv1.PublishSnapshotRequest{EnvironmentId: 5, SnapshotId: 12, Version: 3})
	require.Error(t, err)
	assert.Equal(t, int32(http.StatusConflict), kratoserrors.FromError(err).Code)
}

func TestPublicationServiceNewRejectsNilLogic(t *testing.T) {
	_, err := New(nil)
	assert.Error(t, err)
}

type serviceUserReader struct{ user *model.User }

func (r serviceUserReader) GetByID(context.Context, int64) (*model.User, error) { return r.user, nil }

type serviceTransport struct{ header serviceHeader }

func (serviceTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (serviceTransport) Endpoint() string                  { return "http://localhost" }
func (serviceTransport) Operation() string                 { return "test" }
func (t serviceTransport) RequestHeader() transport.Header { return t.header }
func (t serviceTransport) ReplyHeader() transport.Header   { return serviceHeader{} }

type serviceHeader http.Header

func (h serviceHeader) Get(key string) string { return http.Header(h).Get(key) }
func (h serviceHeader) Set(key, value string) { http.Header(h).Set(key, value) }
func (h serviceHeader) Add(key, value string) { http.Header(h).Add(key, value) }
func (h serviceHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
func (h serviceHeader) Values(key string) []string { return http.Header(h).Values(key) }

func authenticatedServiceContext(t *testing.T, userID int64) context.Context {
	t.Helper()
	manager, err := authjwt.New(config.JWTConfig{
		Issuer: "test", ActiveKID: "primary", AccessTTL: config.Duration{Duration: 15 * time.Minute},
		Keys: map[string]config.Secret{"primary": config.NewSecret("01234567890123456789012345678901")},
	})
	require.NoError(t, err)
	token, _, err := manager.Issue(userID, "session-id")
	require.NoError(t, err)
	header := serviceHeader{}
	header.Set("Authorization", "Bearer "+token)
	transportContext := transport.NewServerContext(context.Background(), serviceTransport{header: header})
	var authenticated context.Context
	next := func(ctx context.Context, _ any) (any, error) {
		authenticated = ctx
		return nil, nil
	}
	_, err = middleware.AdminAuth(manager, serviceUserReader{user: &model.User{Meta: model.Meta{ID: userID}, Enabled: true}})(kratosmiddleware.Handler(next))(transportContext, nil)
	require.NoError(t, err)
	require.NotNil(t, authenticated)
	return authenticated
}
