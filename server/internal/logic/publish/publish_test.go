package publish

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

var (
	errDenied     = errors.New("denied")
	errAudit      = errors.New("audit failed")
	errCache      = errors.New("cache failed")
	errRuntime    = errors.New("runtime version failed")
	fixedTestTime = time.Date(2026, time.August, 17, 5, 6, 7, 0, time.UTC)
)

type publishState struct {
	mu            sync.Mutex
	environmentID int64
	config        model.Config
	snapshots     map[int64]model.Snapshot
	lockOrder     []string
	audits        []model.AuditLog
	failAudit     error
	failRuntime   error
}

func newPublishState(t *testing.T) *publishState {
	t.Helper()
	content := validSnapshotContent(t)
	return &publishState{
		environmentID: 5,
		config: model.Config{
			Meta: model.Meta{ID: 10, Version: 4}, ProjectID: 20, Key: "feature",
			RuntimeVersion: 0,
		},
		snapshots: map[int64]model.Snapshot{
			100: snapshotRecord(100, model.SnapshotStatusUnreleased, 0, content),
			101: snapshotRecord(101, model.SnapshotStatusUnreleased, 0, content),
		},
	}
}

func snapshotRecord(id int64, status model.SnapshotStatus, version int64, content string) model.Snapshot {
	return model.Snapshot{
		Meta: model.Meta{
			ID: id, CreatedBy: 3, UpdatedBy: 3, CreatedAt: fixedTestTime.Add(-time.Hour),
			UpdatedAt: fixedTestTime.Add(-time.Hour), Version: version,
		},
		ProjectID: 20, ConfigID: 10, ConfigKey: "feature", Content: content,
		Description: "snapshot", TagsJSON: "[]", Status: status,
	}
}

func validSnapshotContent(t *testing.T) string {
	t.Helper()
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	content, err := entity.EncodeConfigSnapshot(&entity.ConfigSnapshot{
		Config: &commonv1.Config{
			Id: 10, ProjectId: 20, Key: "feature", Type: fieldType, Value: `"on"`,
		},
		Structures: []*commonv1.Structure{},
		Enums:      []*commonv1.ConfigEnum{},
		Tree:       &commonv1.TreeNode{Value: &commonv1.Field{Key: "feature", Name: "Feature", Type: fieldType}},
	})
	require.NoError(t, err)
	return content
}

func snapshotContentWithMissingEnum(t *testing.T) string {
	t.Helper()
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: "Missing"}}
	content, err := entity.EncodeConfigSnapshot(&entity.ConfigSnapshot{
		Config: &commonv1.Config{
			Id: 10, ProjectId: 20, Key: "feature", Type: fieldType, Value: `"VALUE"`,
		},
		Structures: []*commonv1.Structure{},
		Enums:      []*commonv1.ConfigEnum{},
		Tree:       &commonv1.TreeNode{Value: &commonv1.Field{Key: "feature", Name: "Feature", Type: fieldType}},
	})
	require.NoError(t, err)
	return content
}

func (s *publishState) cloneDatabaseState() (model.Config, map[int64]model.Snapshot) {
	config := s.config
	snapshots := make(map[int64]model.Snapshot, len(s.snapshots))
	for id, snapshot := range s.snapshots {
		snapshots[id] = snapshot
	}
	return config, snapshots
}

type stateTransactor struct{ state *publishState }

func (t stateTransactor) WithTx(_ context.Context, operation func(*xorm.Session) error) error {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	config, snapshots := t.state.cloneDatabaseState()
	if err := operation(&xorm.Session{}); err != nil {
		t.state.config = config
		t.state.snapshots = snapshots
		return err
	}
	return nil
}

type stateConfigRepository struct{ state *publishState }

func (r stateConfigRepository) LockByID(_ context.Context, _ *xorm.Session, environmentID, configID int64) (*model.Config, error) {
	r.state.lockOrder = append(r.state.lockOrder, "config")
	if environmentID != r.state.environmentID || configID != r.state.config.ID {
		return nil, base.Missing("config")
	}
	copy := r.state.config
	return &copy, nil
}

type stateStructureRepository struct{ state *publishState }

func (r stateStructureRepository) ListForConfigTx(_ context.Context, _ *xorm.Session, environmentID, configID int64) ([]model.Structure, error) {
	r.state.lockOrder = append(r.state.lockOrder, "structures")
	if environmentID != r.state.environmentID || configID != r.state.config.ID {
		return nil, base.Missing("config")
	}
	return []model.Structure{}, nil
}

type stateEnumRepository struct{ state *publishState }

func (r stateEnumRepository) ListForConfigTx(_ context.Context, _ *xorm.Session, environmentID, configID int64) ([]model.ConfigEnum, error) {
	r.state.lockOrder = append(r.state.lockOrder, "enums")
	if environmentID != r.state.environmentID || configID != r.state.config.ID {
		return nil, base.Missing("config")
	}
	return []model.ConfigEnum{}, nil
}

type stateSnapshotReader struct{ state *publishState }

func (r stateSnapshotReader) FindByID(_ context.Context, environmentID, snapshotID int64) (*model.Snapshot, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	if environmentID != r.state.environmentID {
		return nil, base.Missing("snapshot")
	}
	snapshot, exists := r.state.snapshots[snapshotID]
	if !exists {
		return nil, base.Missing("snapshot")
	}
	return &snapshot, nil
}

type statePublicationRepository struct{ state *publishState }

func (r statePublicationRepository) LockForConfig(_ context.Context, _ *xorm.Session, environmentID, configID int64) ([]model.Snapshot, error) {
	r.state.lockOrder = append(r.state.lockOrder, "snapshots")
	if environmentID != r.state.environmentID || configID != r.state.config.ID {
		return nil, base.Missing("config")
	}
	ids := make([]int64, 0, len(r.state.snapshots))
	for id, snapshot := range r.state.snapshots {
		if snapshot.ConfigID == configID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	items := make([]model.Snapshot, 0, len(ids))
	for _, id := range ids {
		items = append(items, r.state.snapshots[id])
	}
	return items, nil
}

func (r statePublicationRepository) SetReleased(_ context.Context, _ *xorm.Session, environmentID, configID, snapshotID, expectedVersion, updatedBy int64, changedAt time.Time) error {
	snapshot, exists := r.state.snapshots[snapshotID]
	if !exists || environmentID != r.state.environmentID || snapshot.ConfigID != configID || snapshot.Status != model.SnapshotStatusUnreleased || snapshot.Version != expectedVersion {
		return base.Unchanged("snapshot publication")
	}
	snapshot.Status = model.SnapshotStatusReleased
	snapshot.PublishedAt = &changedAt
	snapshot.PublishedBy = &updatedBy
	snapshot.UpdatedBy = updatedBy
	snapshot.UpdatedAt = changedAt
	snapshot.Version++
	r.state.snapshots[snapshotID] = snapshot
	return nil
}

func (r statePublicationRepository) SetUnreleased(_ context.Context, _ *xorm.Session, environmentID, configID, snapshotID, expectedVersion, updatedBy int64, changedAt time.Time) error {
	snapshot, exists := r.state.snapshots[snapshotID]
	if !exists || environmentID != r.state.environmentID || snapshot.ConfigID != configID || snapshot.Status != model.SnapshotStatusReleased || snapshot.Version != expectedVersion {
		return base.Unchanged("snapshot unpublication")
	}
	snapshot.Status = model.SnapshotStatusUnreleased
	snapshot.PublishedAt = nil
	snapshot.PublishedBy = nil
	snapshot.UpdatedBy = updatedBy
	snapshot.UpdatedAt = changedAt
	snapshot.Version++
	r.state.snapshots[snapshotID] = snapshot
	return nil
}

func (r statePublicationRepository) IncrementRuntimeVersion(_ context.Context, _ *xorm.Session, environmentID, configID int64) error {
	if r.state.failRuntime != nil {
		return r.state.failRuntime
	}
	if environmentID != r.state.environmentID || configID != r.state.config.ID {
		return base.Unchanged("config runtime version")
	}
	r.state.config.RuntimeVersion++
	return nil
}

type stateAuditRepository struct{ state *publishState }

func (r stateAuditRepository) RecordForEnvironmentTx(_ context.Context, _ *xorm.Session, environmentID int64, audit *model.AuditLog) error {
	if r.state.failAudit != nil {
		return r.state.failAudit
	}
	if environmentID != r.state.environmentID {
		return base.Missing("environment")
	}
	r.state.audits = append(r.state.audits, *audit)
	return nil
}

type testAuthorizer struct {
	err   error
	mu    sync.Mutex
	calls [][]string
}

func (a *testAuthorizer) Require(_ context.Context, _ int64, _ int64, keys ...string) error {
	a.mu.Lock()
	a.calls = append(a.calls, append([]string(nil), keys...))
	a.mu.Unlock()
	return a.err
}

type cleanupCall struct {
	environmentID  int64
	projectID      int64
	configKey      string
	runtimeVersion int64
}

type testCache struct {
	mu    sync.Mutex
	err   error
	calls []cleanupCall
}

func (c *testCache) DeletePublishedConfigVersion(_ context.Context, environmentID, projectID int64, configKey string, runtimeVersion int64) error {
	c.mu.Lock()
	c.calls = append(c.calls, cleanupCall{environmentID: environmentID, projectID: projectID, configKey: configKey, runtimeVersion: runtimeVersion})
	c.mu.Unlock()
	return c.err
}

func newTestLogic(t *testing.T, state *publishState, authorizer *testAuthorizer, cache *testCache) *Logic {
	t.Helper()
	logicLayer, err := New(
		stateConfigRepository{state}, stateStructureRepository{state}, stateEnumRepository{state},
		stateSnapshotReader{state}, statePublicationRepository{state}, authorizer,
		stateAuditRepository{state}, stateTransactor{state}, cache,
	)
	require.NoError(t, err)
	logicLayer.now = func() time.Time { return fixedTestTime }
	return logicLayer
}

func TestPublishSwitchesReleaseWithStableLockOrder(t *testing.T) {
	state := newPublishState(t)
	previous := state.snapshots[100]
	previous.Status = model.SnapshotStatusReleased
	previous.Version = 2
	state.snapshots[100] = previous
	target := state.snapshots[101]
	target.Version = 3
	state.snapshots[101] = target
	cache := &testCache{}
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, cache)

	result, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7, RequestID: "request-1"}, 5, 101, 3)

	require.NoError(t, err)
	assert.Equal(t, model.SnapshotStatusReleased, result.Status)
	assert.Equal(t, int64(4), result.Version)
	require.NotNil(t, result.PublishedBy)
	assert.Equal(t, int64(7), *result.PublishedBy)
	assert.Equal(t, model.SnapshotStatusUnreleased, state.snapshots[100].Status)
	assert.Equal(t, int64(3), state.snapshots[100].Version)
	assert.Equal(t, int64(1), state.config.RuntimeVersion)
	assert.Equal(t, []string{"config", "structures", "enums", "snapshots"}, state.lockOrder)
	require.Len(t, state.audits, 1)
	assert.Equal(t, "snapshot.publish", state.audits[0].Action)
	assert.Equal(t, "request-1", state.audits[0].RequestID)
	require.Len(t, cache.calls, 1)
	assert.Equal(t, cleanupCall{environmentID: 5, projectID: 20, configKey: "feature", runtimeVersion: 0}, cache.calls[0])
}

func TestConcurrentPublishLeavesExactlyOneReleasedSnapshot(t *testing.T) {
	state := newPublishState(t)
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, &testCache{})
	errorsByCall := make(chan error, 2)
	var group sync.WaitGroup
	for _, snapshotID := range []int64{100, 101} {
		group.Add(1)
		go func(id int64) {
			defer group.Done()
			_, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, id, 0)
			errorsByCall <- err
		}(snapshotID)
	}
	group.Wait()
	close(errorsByCall)
	for err := range errorsByCall {
		require.NoError(t, err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	released := 0
	for _, snapshot := range state.snapshots {
		if snapshot.Status == model.SnapshotStatusReleased {
			released++
		}
	}
	assert.Equal(t, 1, released)
	assert.Equal(t, int64(2), state.config.RuntimeVersion)
	assert.Len(t, state.audits, 2)
	assert.Equal(t, []string{
		"config", "structures", "enums", "snapshots",
		"config", "structures", "enums", "snapshots",
	}, state.lockOrder)
}

func TestPublishRejectsRepeatedAndStaleOperations(t *testing.T) {
	tests := []struct {
		name      string
		status    model.SnapshotStatus
		version   int64
		expected  uint32
		unpublish bool
	}{
		{name: "already released", status: model.SnapshotStatusReleased, version: 2, expected: 2},
		{name: "already unreleased", status: model.SnapshotStatusUnreleased, version: 2, expected: 2, unpublish: true},
		{name: "stale publish", status: model.SnapshotStatusUnreleased, version: 2, expected: 1},
		{name: "stale unpublish", status: model.SnapshotStatusReleased, version: 2, expected: 1, unpublish: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newPublishState(t)
			target := state.snapshots[100]
			target.Status, target.Version = test.status, test.version
			state.snapshots[100] = target
			cache := &testCache{}
			logicLayer := newTestLogic(t, state, &testAuthorizer{}, cache)
			var err error
			if test.unpublish {
				_, err = logicLayer.Unpublish(context.Background(), permission.Actor{UserID: 7}, 5, 100, test.expected)
			} else {
				_, err = logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, 100, test.expected)
			}
			assert.ErrorIs(t, err, entity.ErrConflict)
			assert.Equal(t, int64(0), state.config.RuntimeVersion)
			assert.Empty(t, state.audits)
			assert.Empty(t, cache.calls)
		})
	}
}

func TestPublishRejectsPermissionAndForeignEnvironment(t *testing.T) {
	state := newPublishState(t)
	authorizer := &testAuthorizer{err: errDenied}
	logicLayer := newTestLogic(t, state, authorizer, &testCache{})

	_, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, 100, 0)
	assert.ErrorIs(t, err, errDenied)
	assert.Empty(t, state.lockOrder)
	require.Len(t, authorizer.calls, 1)
	assert.Equal(t, []string{permission.SnapshotPublish}, authorizer.calls[0])

	authorizer.err = nil
	_, err = logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 99, 100, 0)
	assert.ErrorIs(t, err, base.ErrNotFound)
	assert.Empty(t, state.lockOrder)
}

func TestPublishRejectsInvalidSnapshotBeforeWrites(t *testing.T) {
	tests := []struct {
		name    string
		content func(*testing.T) string
	}{
		{name: "malformed content", content: func(*testing.T) string { return `{"config":` }},
		{name: "missing enum reference", content: snapshotContentWithMissingEnum},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newPublishState(t)
			target := state.snapshots[100]
			target.Content = test.content(t)
			state.snapshots[100] = target
			logicLayer := newTestLogic(t, state, &testAuthorizer{}, &testCache{})

			_, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, 100, 0)

			assert.ErrorIs(t, err, entity.ErrInvalid)
			assert.Equal(t, model.SnapshotStatusUnreleased, state.snapshots[100].Status)
			assert.Equal(t, int64(0), state.config.RuntimeVersion)
			assert.Empty(t, state.audits)
		})
	}
}

func TestPublishRejectsMultipleReleasedSnapshots(t *testing.T) {
	state := newPublishState(t)
	first := state.snapshots[100]
	first.Status = model.SnapshotStatusReleased
	state.snapshots[100] = first
	second := state.snapshots[101]
	second.Status = model.SnapshotStatusReleased
	state.snapshots[101] = second
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, &testCache{})

	_, err := logicLayer.Unpublish(context.Background(), permission.Actor{UserID: 7}, 5, 100, 0)

	assert.ErrorIs(t, err, entity.ErrConflict)
	assert.Equal(t, int64(0), state.config.RuntimeVersion)
	assert.Empty(t, state.audits)
}

func TestPublishRejectsUnsupportedSnapshotStatus(t *testing.T) {
	state := newPublishState(t)
	target := state.snapshots[100]
	target.Status = model.SnapshotStatus(2)
	state.snapshots[100] = target
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, &testCache{})

	_, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, 100, 0)

	assert.ErrorIs(t, err, entity.ErrConflict)
	assert.Equal(t, int64(0), state.config.RuntimeVersion)
	assert.Empty(t, state.audits)
}

func TestPublishRollsBackStateAndAuditTogether(t *testing.T) {
	state := newPublishState(t)
	previous := state.snapshots[101]
	previous.Status = model.SnapshotStatusReleased
	previous.Version = 2
	state.snapshots[101] = previous
	state.failAudit = errAudit
	cache := &testCache{}
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, cache)

	_, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, 100, 0)

	assert.ErrorIs(t, err, errAudit)
	assert.Equal(t, model.SnapshotStatusUnreleased, state.snapshots[100].Status)
	assert.Equal(t, int64(0), state.snapshots[100].Version)
	assert.Equal(t, model.SnapshotStatusReleased, state.snapshots[101].Status)
	assert.Equal(t, int64(2), state.snapshots[101].Version)
	assert.Equal(t, int64(0), state.config.RuntimeVersion)
	assert.Empty(t, state.audits)
	assert.Empty(t, cache.calls)
}

func TestPublishRollsBackWhenRuntimeVersionCannotAdvance(t *testing.T) {
	state := newPublishState(t)
	state.failRuntime = errRuntime
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, &testCache{})

	_, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, 100, 0)

	assert.ErrorIs(t, err, errRuntime)
	assert.Equal(t, model.SnapshotStatusUnreleased, state.snapshots[100].Status)
	assert.Equal(t, int64(0), state.snapshots[100].Version)
	assert.Equal(t, int64(0), state.config.RuntimeVersion)
	assert.Empty(t, state.audits)
}

func TestCacheCleanupFailureDoesNotChangeCommittedResult(t *testing.T) {
	state := newPublishState(t)
	cache := &testCache{err: errCache}
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, cache)

	result, err := logicLayer.Publish(context.Background(), permission.Actor{UserID: 7}, 5, 100, 0)

	require.NoError(t, err)
	assert.Equal(t, model.SnapshotStatusReleased, result.Status)
	assert.Equal(t, model.SnapshotStatusReleased, state.snapshots[100].Status)
	assert.Equal(t, int64(1), state.config.RuntimeVersion)
	require.Len(t, state.audits, 1)
	require.Len(t, cache.calls, 1)
}

func TestUnpublishCommitsExplicitTargetAndRuntimeVersion(t *testing.T) {
	state := newPublishState(t)
	target := state.snapshots[100]
	target.Status = model.SnapshotStatusReleased
	target.Version = 3
	target.PublishedAt = &fixedTestTime
	publisher := int64(6)
	target.PublishedBy = &publisher
	state.snapshots[100] = target
	cache := &testCache{}
	logicLayer := newTestLogic(t, state, &testAuthorizer{}, cache)

	result, err := logicLayer.Unpublish(context.Background(), permission.Actor{UserID: 7, RequestID: "request-2"}, 5, 100, 3)

	require.NoError(t, err)
	assert.Equal(t, model.SnapshotStatusUnreleased, result.Status)
	assert.Nil(t, result.PublishedAt)
	assert.Nil(t, result.PublishedBy)
	assert.Equal(t, int64(4), result.Version)
	assert.Equal(t, int64(1), state.config.RuntimeVersion)
	require.Len(t, state.audits, 1)
	assert.Equal(t, "snapshot.unpublish", state.audits[0].Action)
	require.Len(t, cache.calls, 1)
}

func TestNewRejectsMissingDependency(t *testing.T) {
	_, err := New(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	assert.Error(t, err)
}
