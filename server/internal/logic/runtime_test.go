package logic

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type runtimeState struct {
	mu            sync.Mutex
	key           model.ProjectAPIKey
	project       model.Project
	projectError  error
	config        model.Config
	snapshot      model.Snapshot
	configReads   int
	snapshotReads int
	used          int
}

func (s *runtimeState) tx(_ context.Context, operation func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return operation(context.Background())
}

type stateRepository struct {
	repository.ProjectAPIKeyRepository
	state *runtimeState
}

func (r stateRepository) LockRuntimeCredential(_ context.Context, secretHash []byte) (*model.ProjectAPIKey, error) {
	if !bytes.Equal(secretHash, r.state.key.SecretHash) {
		return nil, repository.Missing("runtime API key")
	}
	clone := r.state.key
	clone.SecretHash = append([]byte(nil), r.state.key.SecretHash...)
	return &clone, nil
}

func (r stateRepository) FindRuntimeProject(context.Context, int64, int64) (*model.Project, error) {
	clone := r.state.project
	return &clone, r.state.projectError
}

func (r stateRepository) FindRuntimeConfig(_ context.Context, environmentID, projectID int64, key string) (*model.Config, error) {
	r.state.configReads++
	if environmentID != r.state.config.EnvironmentID || projectID != r.state.config.ProjectID || key != r.state.config.Key {
		return nil, repository.Missing("runtime config")
	}
	clone := r.state.config
	return &clone, nil
}

func (r stateRepository) FindReleasedSnapshot(context.Context, int64, int64) (*model.Snapshot, error) {
	r.state.snapshotReads++
	clone := r.state.snapshot
	return &clone, nil
}

func (r stateRepository) MarkUsed(context.Context, []byte, time.Time) (rowsAffected int64, err error) {
	r.state.mu.Lock()
	r.state.used++
	r.state.mu.Unlock()
	return
}

func snapshotContent(t *testing.T, value string) string {
	t.Helper()
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	content, err := entity.EncodeConfigSnapshot(&entity.ConfigSnapshot{
		Config: &commonv1.Config{
			Id:        10,
			ProjectId: 20,
			Key:       "feature",
			Type:      fieldType,
			Value:     value,
		},
		Tree: &commonv1.TreeNode{
			Value: &commonv1.Field{
				Key:  "feature",
				Name: "Feature",
				Type: fieldType,
			},
		},
	})
	require.NoError(t, err)
	return content
}

func newRuntimeFixture(t *testing.T) (*credential.Manager, *runtimeState, string, *ContentCache) {
	t.Helper()
	manager, err := credential.New("01234567890123456789012345678901")
	require.NoError(t, err)
	generated, err := manager.Generate()
	require.NoError(t, err)
	state := &runtimeState{
		key: model.ProjectAPIKey{
			RecordMeta: model.RecordMeta{ID: 30},
			ProjectID:  20,
			SecretHash: generated.Hash,
		},
		project: model.Project{
			Meta: model.Meta{ID: 20},
			Key:  "website",
		},
		config: model.Config{
			Meta:           model.Meta{ID: 10},
			ProjectID:      20,
			Key:            "feature",
			RuntimeVersion: 1,
		},
		snapshot: model.Snapshot{
			Meta:      model.Meta{ID: 40},
			ProjectID: 20,
			ConfigID:  10,
			ConfigKey: "feature",
			Status:    model.SnapshotStatusReleased,
			Content:   snapshotContent(t, `"one"`),
		},
	}
	contentCache := NewContentCache(cache.NewMemory())
	return manager, state, generated.Full, contentCache
}

func newRuntimeLogic(t *testing.T, manager *credential.Manager, state *runtimeState, contentCache *ContentCache) *RuntimeLogic {
	t.Helper()
	store := transactionFunc(state.tx)
	return NewRuntimeLogic(stateRepository{state: state}, manager, store, contentCache)
}

func TestRuntimeReadsDatabaseVersionBeforeSharedCache(t *testing.T) {
	manager, state, full, contentCache := newRuntimeFixture(t)
	firstInstance := newRuntimeLogic(t, manager, state, contentCache)
	secondInstance := newRuntimeLogic(t, manager, state, contentCache)

	first, err := firstInstance.Read(context.Background(), full, "feature")
	require.NoError(t, err)
	assert.Equal(t, `"one"`, first.Content)
	assert.Equal(t, uint64(1), first.Version)

	state.mu.Lock()
	state.config.RuntimeVersion = 2
	state.snapshot.Content = snapshotContent(t, `"two"`)
	state.mu.Unlock()
	second, err := secondInstance.Read(context.Background(), full, "feature")
	require.NoError(t, err)
	assert.Equal(t, `"two"`, second.Content)
	assert.Equal(t, uint64(2), second.Version)

	state.mu.Lock()
	assert.Equal(t, 2, state.configReads, "every request reads the database runtime version")
	assert.Equal(t, 2, state.snapshotReads, "the new version cannot hit the old instance cache entry")
	state.mu.Unlock()
}

func TestRuntimeRejectsRotatedAndRevokedKeys(t *testing.T) {
	manager, state, oldFull, contentCache := newRuntimeFixture(t)
	logicLayer := newRuntimeLogic(t, manager, state, contentCache)

	replacement, err := manager.Generate()
	require.NoError(t, err)
	state.mu.Lock()
	state.key.SecretHash = replacement.Hash
	state.mu.Unlock()
	_, err = logicLayer.Read(context.Background(), oldFull, "feature")
	assert.ErrorIs(t, err, ErrUnauthenticated)
	_, err = logicLayer.Read(context.Background(), replacement.Full, "feature")
	require.NoError(t, err)

	revokedAt := time.Now().UTC()
	state.mu.Lock()
	state.key.RevokedAt = &revokedAt
	state.mu.Unlock()
	_, err = logicLayer.Read(context.Background(), replacement.Full, "feature")
	assert.ErrorIs(t, err, ErrUnauthenticated)
}

func TestContentCacheCleanerDeletesOnlyRequestedVersion(t *testing.T) {
	manager, state, full, contentCache := newRuntimeFixture(t)
	logicLayer := newRuntimeLogic(t, manager, state, contentCache)
	_, err := logicLayer.Read(context.Background(), full, "feature")
	require.NoError(t, err)
	require.NoError(t, contentCache.DeletePublishedConfigVersion(context.Background(), state.config.ID, "feature", 1))

	_, err = logicLayer.Read(context.Background(), full, "feature")
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

	_, err = newRuntimeLogic(t, manager, state, contentCache).Read(context.Background(), full, "feature")
	assert.ErrorContains(t, err, "scope does not match")
}

func TestRuntimeCacheIsolatesSameProjectAndKeyAcrossEnvironments(t *testing.T) {
	manager, first, full, shared := newRuntimeFixture(t)
	_, second, _, _ := newRuntimeFixture(t)
	second.key = first.key
	second.key.EnvironmentID = 2
	second.config.EnvironmentID = 2
	second.config.ID = first.config.ID + 1
	firstLogic := newRuntimeLogic(t, manager, first, shared)
	secondLogic := newRuntimeLogic(t, manager, second, shared)
	_, err := firstLogic.Read(context.Background(), full, first.config.Key)
	require.NoError(t, err)
	// An invalid sibling snapshot must be examined, never bypassed by the first environment's cache.
	second.snapshot.Content = "invalid snapshot"
	_, err = secondLogic.Read(context.Background(), full, second.config.Key)
	require.Error(t, err)
	require.Equal(t, 1, second.snapshotReads)
}

func TestRuntimeCredentialDeterminesProjectAndEnvironment(t *testing.T) {
	manager, first, firstSecret, shared := newRuntimeFixture(t)
	_, second, secondSecret, _ := newRuntimeFixture(t)
	second.key.ProjectID = 21
	second.key.EnvironmentID = 2
	second.project.ID = 21
	second.project.Key = "other-project"
	second.config.ProjectID = 21
	second.config.EnvironmentID = 2
	second.config.ID = 11
	decoded, err := entity.DecodeConfigSnapshot(second.snapshot.Content)
	require.NoError(t, err)
	decoded.Config.Id = second.config.ID
	decoded.Config.ProjectId = second.config.ProjectID
	decoded.Config.Value = `"other-environment"`
	second.snapshot.Content, err = entity.EncodeConfigSnapshot(decoded)
	require.NoError(t, err)
	firstLogic := newRuntimeLogic(t, manager, first, shared)
	secondLogic := newRuntimeLogic(t, manager, second, shared)
	firstResult, err := firstLogic.Read(context.Background(), firstSecret, "feature")
	require.NoError(t, err)
	secondResult, err := secondLogic.Read(context.Background(), secondSecret, "feature")
	require.NoError(t, err)
	assert.Equal(t, `"one"`, firstResult.Content)
	assert.Equal(t, `"other-environment"`, secondResult.Content)
	_, err = secondLogic.Read(context.Background(), firstSecret, "feature")
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestRuntimeRejectsUnavailableCredentialScopeBeforeCache(t *testing.T) {
	manager, state, secret, shared := newRuntimeFixture(t)
	runtime := newRuntimeLogic(t, manager, state, shared)
	_, err := runtime.Read(context.Background(), secret, "feature")
	require.NoError(t, err)
	state.projectError = repository.Missing("runtime project")
	_, err = runtime.Read(context.Background(), secret, "feature")
	require.ErrorIs(t, err, ErrUnauthenticated)
	assert.Equal(t, 1, state.configReads)
}
