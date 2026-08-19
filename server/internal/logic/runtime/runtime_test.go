package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/cache"
)

type runtimeState struct {
	mu            sync.Mutex
	key           model.ProjectAPIKey
	project       model.Project
	config        model.Config
	snapshot      model.Snapshot
	configReads   int
	snapshotReads int
	used          int
}

type stateTransactor struct{ state *runtimeState }

func (t stateTransactor) WithTx(_ context.Context, operation func(*xorm.Session) error) error {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	return operation(&xorm.Session{})
}

type stateRepository struct{ state *runtimeState }

func (r stateRepository) LockRuntimeCredential(_ context.Context, _ *xorm.Session, publicID string) (*model.ProjectAPIKey, error) {
	if publicID != r.state.key.PublicID {
		return nil, base.Missing("runtime API key")
	}
	clone := r.state.key
	clone.SecretHash = append([]byte(nil), r.state.key.SecretHash...)
	return &clone, nil
}

func (r stateRepository) FindRuntimeProjectTx(context.Context, *xorm.Session, int64) (*model.Project, error) {
	clone := r.state.project
	return &clone, nil
}

func (r stateRepository) FindRuntimeConfigTx(_ context.Context, _ *xorm.Session, projectID int64, key string) (*model.Config, error) {
	r.state.configReads++
	if projectID != r.state.config.ProjectID || key != r.state.config.Key {
		return nil, base.Missing("runtime config")
	}
	clone := r.state.config
	return &clone, nil
}

func (r stateRepository) FindReleasedSnapshotTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error) {
	r.state.snapshotReads++
	clone := r.state.snapshot
	return &clone, nil
}

func (r stateRepository) MarkUsed(context.Context, string, time.Time) error {
	r.state.mu.Lock()
	r.state.used++
	r.state.mu.Unlock()
	return nil
}

func snapshotContent(t *testing.T, value string) string {
	t.Helper()
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	content, err := entity.EncodeConfigSnapshot(&entity.ConfigSnapshot{
		Config: &commonv1.Config{Id: 10, ProjectId: 20, Key: "feature", Type: fieldType, Value: value},
		Tree:   &commonv1.TreeNode{Value: &commonv1.Field{Key: "feature", Name: "Feature", Type: fieldType}},
	})
	require.NoError(t, err)
	return content
}

func newRuntimeFixture(t *testing.T) (*credential.Manager, *runtimeState, string, *ContentCache) {
	t.Helper()
	manager, err := credential.New(config.NewSecret("01234567890123456789012345678901"))
	require.NoError(t, err)
	generated, err := manager.Generate()
	require.NoError(t, err)
	state := &runtimeState{
		key:      model.ProjectAPIKey{RecordMeta: model.RecordMeta{ID: 30}, ProjectID: 20, PublicID: generated.PublicID, SecretHash: generated.Hash},
		project:  model.Project{Meta: model.Meta{ID: 20}, EnvironmentID: 5, Key: "website"},
		config:   model.Config{Meta: model.Meta{ID: 10}, ProjectID: 20, Key: "feature", RuntimeVersion: 1},
		snapshot: model.Snapshot{Meta: model.Meta{ID: 40}, ProjectID: 20, ConfigID: 10, ConfigKey: "feature", Status: model.SnapshotStatusReleased, Content: snapshotContent(t, `"one"`)},
	}
	contentCache, err := NewContentCache(cache.NewMemory())
	require.NoError(t, err)
	return manager, state, generated.Full, contentCache
}

func newRuntimeLogic(t *testing.T, manager *credential.Manager, state *runtimeState, contentCache *ContentCache) *Logic {
	t.Helper()
	logicLayer, err := New(stateRepository{state}, manager, stateTransactor{state}, contentCache)
	require.NoError(t, err)
	return logicLayer
}

func TestRuntimeReadsDatabaseVersionBeforeSharedCache(t *testing.T) {
	manager, state, full, contentCache := newRuntimeFixture(t)
	firstInstance := newRuntimeLogic(t, manager, state, contentCache)
	secondInstance := newRuntimeLogic(t, manager, state, contentCache)

	first, err := firstInstance.Read(context.Background(), full, "website", "feature")
	require.NoError(t, err)
	assert.Equal(t, `"one"`, first.Content)
	assert.Equal(t, uint64(1), first.Version)

	state.mu.Lock()
	state.config.RuntimeVersion = 2
	state.snapshot.Content = snapshotContent(t, `"two"`)
	state.mu.Unlock()
	second, err := secondInstance.Read(context.Background(), full, "website", "feature")
	require.NoError(t, err)
	assert.Equal(t, `"two"`, second.Content)
	assert.Equal(t, uint64(2), second.Version)

	state.mu.Lock()
	assert.Equal(t, 2, state.configReads, "every request reads the database runtime version")
	assert.Equal(t, 2, state.snapshotReads, "the new version cannot hit the old instance cache entry")
	state.mu.Unlock()
}

func TestRuntimeRejectsWrongProjectRotatedAndRevokedKeys(t *testing.T) {
	manager, state, oldFull, contentCache := newRuntimeFixture(t)
	logicLayer := newRuntimeLogic(t, manager, state, contentCache)

	_, err := logicLayer.Read(context.Background(), oldFull, "another-project", "feature")
	assert.ErrorIs(t, err, ErrProjectMismatch)

	replacement, err := manager.Generate()
	require.NoError(t, err)
	state.mu.Lock()
	state.key.PublicID = replacement.PublicID
	state.key.SecretHash = replacement.Hash
	state.mu.Unlock()
	_, err = logicLayer.Read(context.Background(), oldFull, "website", "feature")
	assert.ErrorIs(t, err, ErrUnauthenticated)
	_, err = logicLayer.Read(context.Background(), replacement.Full, "website", "feature")
	require.NoError(t, err)

	revokedAt := time.Now().UTC()
	state.mu.Lock()
	state.key.RevokedAt = &revokedAt
	state.mu.Unlock()
	_, err = logicLayer.Read(context.Background(), replacement.Full, "website", "feature")
	assert.ErrorIs(t, err, ErrUnauthenticated)
}

func TestContentCacheCleanerDeletesOnlyRequestedVersion(t *testing.T) {
	manager, state, full, contentCache := newRuntimeFixture(t)
	logicLayer := newRuntimeLogic(t, manager, state, contentCache)
	_, err := logicLayer.Read(context.Background(), full, "website", "feature")
	require.NoError(t, err)
	require.NoError(t, contentCache.DeletePublishedConfigVersion(context.Background(), 5, 20, "feature", 1))

	_, err = logicLayer.Read(context.Background(), full, "website", "feature")
	require.NoError(t, err)
	state.mu.Lock()
	assert.Equal(t, 2, state.snapshotReads)
	state.mu.Unlock()
}

func TestRuntimeRejectsSnapshotWhoseEmbeddedConfigHasDifferentScope(t *testing.T) {
	manager, state, full, contentCache := newRuntimeFixture(t)
	state.snapshot.Content = snapshotContent(t, `"value"`)
	decoded, err := entity.DecodeConfigSnapshot(state.snapshot.Content)
	require.NoError(t, err)
	decoded.Config.Id = 999
	state.snapshot.Content, err = entity.EncodeConfigSnapshot(decoded)
	require.NoError(t, err)

	_, err = newRuntimeLogic(t, manager, state, contentCache).Read(context.Background(), full, "website", "feature")
	assert.ErrorContains(t, err, "scope does not match")
}
