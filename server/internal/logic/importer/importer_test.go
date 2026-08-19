package importer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

var errAudit = errors.New("audit failed")

type importState struct {
	mu              sync.Mutex
	projects        map[int64]model.Project
	configs         map[int64]model.Config
	structures      map[int64][]model.Structure
	enums           map[int64][]model.ConfigEnum
	snapshots       map[int64]model.Snapshot
	snapshotEnv     map[int64]int64
	records         map[string]model.ImportRecord
	audits          []model.AuditLog
	nextConfigID    int64
	nextStructureID int64
	nextEnumID      int64
	nextSnapshotID  int64
	nextRecordID    int64
	failAudit       error
}

type stateCopy struct {
	projects        map[int64]model.Project
	configs         map[int64]model.Config
	structures      map[int64][]model.Structure
	enums           map[int64][]model.ConfigEnum
	snapshots       map[int64]model.Snapshot
	snapshotEnv     map[int64]int64
	records         map[string]model.ImportRecord
	audits          []model.AuditLog
	nextConfigID    int64
	nextStructureID int64
	nextEnumID      int64
	nextSnapshotID  int64
	nextRecordID    int64
}

func newImportState(t *testing.T) *importState {
	t.Helper()
	return &importState{
		projects: map[int64]model.Project{
			11: {Meta: model.Meta{ID: 11}, EnvironmentID: 1, Key: "source"},
			20: {Meta: model.Meta{ID: 20}, EnvironmentID: 2, Key: "target"},
		},
		configs: map[int64]model.Config{}, structures: map[int64][]model.Structure{}, enums: map[int64][]model.ConfigEnum{},
		snapshots: map[int64]model.Snapshot{
			12: {Meta: model.Meta{ID: 12}, ProjectID: 11, ConfigID: 10, ConfigKey: "feature", Content: sourceContent(t), Status: model.SnapshotStatusUnreleased},
		},
		snapshotEnv: map[int64]int64{12: 1}, records: map[string]model.ImportRecord{},
		nextConfigID: 100, nextStructureID: 200, nextEnumID: 300, nextSnapshotID: 400, nextRecordID: 500,
	}
}

func sourceContent(t *testing.T) string {
	t.Helper()
	structureType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_StructureKey{StructureKey: "Profile"}}
	enumType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: "Status"}}
	content, err := entity.EncodeConfigSnapshot(&entity.ConfigSnapshot{
		Config:     &commonv1.Config{Id: 10, ProjectId: 11, Key: "feature", Description: "source config", Type: structureType, Value: `{"status":"ACTIVE"}`},
		Structures: []*commonv1.Structure{{Id: 101, ConfigId: 10, Key: "Profile", Name: "Profile", Fields: []*commonv1.Field{{Key: "status", Name: "Status", Type: enumType}}}},
		Enums:      []*commonv1.ConfigEnum{{Id: 201, ConfigId: 10, Key: "Status", Name: "Status", Values: []*commonv1.SelectOption{{Label: "Active", Value: "ACTIVE"}}}},
		Tree:       &commonv1.TreeNode{Value: &commonv1.Field{Key: "feature", Name: "Config", Type: structureType}},
	})
	require.NoError(t, err)
	return content
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	result := make(map[K]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (s *importState) snapshot() stateCopy {
	structures := make(map[int64][]model.Structure, len(s.structures))
	for key, value := range s.structures {
		structures[key] = append([]model.Structure(nil), value...)
	}
	enums := make(map[int64][]model.ConfigEnum, len(s.enums))
	for key, value := range s.enums {
		enums[key] = append([]model.ConfigEnum(nil), value...)
	}
	return stateCopy{
		projects: cloneMap(s.projects), configs: cloneMap(s.configs), structures: structures, enums: enums,
		snapshots: cloneMap(s.snapshots), snapshotEnv: cloneMap(s.snapshotEnv), records: cloneMap(s.records), audits: append([]model.AuditLog(nil), s.audits...),
		nextConfigID: s.nextConfigID, nextStructureID: s.nextStructureID, nextEnumID: s.nextEnumID,
		nextSnapshotID: s.nextSnapshotID, nextRecordID: s.nextRecordID,
	}
}

func (s *importState) restore(copy stateCopy) {
	s.projects, s.configs, s.structures, s.enums = copy.projects, copy.configs, copy.structures, copy.enums
	s.snapshots, s.snapshotEnv, s.records, s.audits = copy.snapshots, copy.snapshotEnv, copy.records, copy.audits
	s.nextConfigID, s.nextStructureID, s.nextEnumID = copy.nextConfigID, copy.nextStructureID, copy.nextEnumID
	s.nextSnapshotID, s.nextRecordID = copy.nextSnapshotID, copy.nextRecordID
}

type stateTransactor struct{ state *importState }

func (t stateTransactor) WithTx(_ context.Context, operation func(*xorm.Session) error) error {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	before := t.state.snapshot()
	if err := operation(&xorm.Session{}); err != nil {
		t.state.restore(before)
		return err
	}
	return nil
}

type stateImports struct{ state *importState }

func importRecordKey(record *model.ImportRecord) string {
	return fmt.Sprintf("%d:%d:%s", record.UserID, record.TargetEnvironmentID, record.IdempotencyKey)
}

func (r stateImports) ClaimTx(_ context.Context, _ *xorm.Session, record *model.ImportRecord) (*model.ImportRecord, bool, error) {
	key := importRecordKey(record)
	if existing, ok := r.state.records[key]; ok {
		clone := existing
		return &clone, false, nil
	}
	if r.state.snapshotEnv[record.SourceSnapshotID] != record.SourceEnvironmentID || r.state.projects[record.TargetProjectID].EnvironmentID != record.TargetEnvironmentID {
		return nil, false, base.Missing("import scope")
	}
	clone := *record
	clone.ID = r.state.nextRecordID
	r.state.nextRecordID++
	clone.RequestHash = append([]byte(nil), record.RequestHash...)
	r.state.records[key] = clone
	return &clone, true, nil
}

func (r stateImports) CompleteTx(_ context.Context, _ *xorm.Session, recordID, targetSnapshotID int64, resultJSON string) error {
	for key, record := range r.state.records {
		if record.ID == recordID {
			record.Status = model.ImportStatusSucceeded
			record.TargetSnapshotID = &targetSnapshotID
			record.ResultJSON = &resultJSON
			r.state.records[key] = record
			return nil
		}
	}
	return base.Missing("import record")
}

type stateProjects struct{ state *importState }

func (r stateProjects) LockByID(_ context.Context, _ *xorm.Session, environmentID, projectID int64) (*model.Project, error) {
	item, ok := r.state.projects[projectID]
	if !ok || item.EnvironmentID != environmentID {
		return nil, base.Missing("project")
	}
	return &item, nil
}

type stateConfigs struct{ state *importState }

func (r stateConfigs) CreateTx(_ context.Context, _ *xorm.Session, environmentID, projectID int64, item *model.Config) error {
	if r.state.projects[projectID].EnvironmentID != environmentID {
		return base.Missing("project")
	}
	for _, existing := range r.state.configs {
		if existing.ProjectID == projectID && existing.Key == item.Key && existing.DeletedAt.IsZero() {
			return repository.ErrKeyConflict
		}
	}
	item.ID = r.state.nextConfigID
	r.state.nextConfigID++
	item.ProjectID = projectID
	r.state.configs[item.ID] = *item
	return nil
}

func (r stateConfigs) LockByID(_ context.Context, _ *xorm.Session, environmentID, configID int64) (*model.Config, error) {
	item, ok := r.state.configs[configID]
	project, projectOK := r.state.projects[item.ProjectID]
	if !ok || !projectOK || project.EnvironmentID != environmentID {
		return nil, base.Missing("config")
	}
	return &item, nil
}

func (r stateConfigs) UpdateTx(_ context.Context, _ *xorm.Session, _ int64, configID int64, update repository.ConfigUpdate) error {
	item := r.state.configs[configID]
	if item.Version != update.Version {
		return base.ErrNoRowsAffected
	}
	item.Description, item.IsArray, item.TypeJSON, item.UpdatedBy = update.Description, update.IsArray, update.TypeJSON, update.UpdatedBy
	item.Version++
	r.state.configs[configID] = item
	return nil
}

func (r stateConfigs) UpdateValueTx(_ context.Context, _ *xorm.Session, _ int64, configID int64, update repository.ConfigValueUpdate) error {
	item := r.state.configs[configID]
	if item.Version != update.Version {
		return base.ErrNoRowsAffected
	}
	item.Value, item.UpdatedBy = update.Value, update.UpdatedBy
	item.Version++
	r.state.configs[configID] = item
	return nil
}

type stateStructures struct{ state *importState }

func (r stateStructures) ReconcileTx(_ context.Context, _ *xorm.Session, _ int64, configID int64, items []*model.Structure, _ int64) error {
	result := make([]model.Structure, 0, len(items))
	for _, item := range items {
		item.ID = r.state.nextStructureID
		r.state.nextStructureID++
		result = append(result, *item)
	}
	r.state.structures[configID] = result
	return nil
}

type stateEnums struct{ state *importState }

func (r stateEnums) ReconcileTx(_ context.Context, _ *xorm.Session, _ int64, configID int64, items []*model.ConfigEnum, _ int64) error {
	result := make([]model.ConfigEnum, 0, len(items))
	for _, item := range items {
		item.ID = r.state.nextEnumID
		r.state.nextEnumID++
		result = append(result, *item)
	}
	r.state.enums[configID] = result
	return nil
}

type stateSnapshots struct{ state *importState }

func (r stateSnapshots) FindByID(_ context.Context, environmentID, snapshotID int64) (*model.Snapshot, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	item, ok := r.state.snapshots[snapshotID]
	if !ok || r.state.snapshotEnv[snapshotID] != environmentID {
		return nil, base.Missing("snapshot")
	}
	return &item, nil
}

func (r stateSnapshots) LockByID(_ context.Context, _ *xorm.Session, environmentID, snapshotID int64) (*model.Snapshot, error) {
	item, ok := r.state.snapshots[snapshotID]
	if !ok || r.state.snapshotEnv[snapshotID] != environmentID {
		return nil, base.Missing("snapshot")
	}
	return &item, nil
}

func (r stateSnapshots) CreateTx(_ context.Context, _ *xorm.Session, environmentID, projectID, configID int64, item *model.Snapshot) error {
	item.ID = r.state.nextSnapshotID
	r.state.nextSnapshotID++
	item.ProjectID, item.ConfigID, item.Status = projectID, configID, model.SnapshotStatusUnreleased
	r.state.snapshots[item.ID] = *item
	r.state.snapshotEnv[item.ID] = environmentID
	return nil
}

func (r stateSnapshots) SetCurrent(_ context.Context, _ *xorm.Session, environmentID, configID, snapshotID, _ int64) error {
	for id, item := range r.state.snapshots {
		if r.state.snapshotEnv[id] == environmentID && item.ConfigID == configID {
			item.IsUsing = id == snapshotID
			r.state.snapshots[id] = item
		}
	}
	return nil
}

type statePermissions struct {
	mu     sync.Mutex
	denied map[string]bool
	checks []string
}

func (p *statePermissions) Require(_ context.Context, _ int64, environmentID int64, required ...string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, key := range required {
		p.checks = append(p.checks, fmt.Sprintf("%d:%s", environmentID, key))
		if p.denied[fmt.Sprintf("%d:%s", environmentID, key)] {
			return permission.ErrForbidden
		}
	}
	return nil
}

type stateAudits struct{ state *importState }

func (a stateAudits) RecordForEnvironmentTx(_ context.Context, _ *xorm.Session, _ int64, item *model.AuditLog) error {
	if a.state.failAudit != nil {
		return a.state.failAudit
	}
	a.state.audits = append(a.state.audits, *item)
	return nil
}

type cleanupCall struct {
	environmentID, projectID, version int64
	key                               string
}
type stateCache struct {
	mu    sync.Mutex
	calls []cleanupCall
}

func (c *stateCache) DeletePublishedConfigVersion(_ context.Context, environmentID, projectID int64, key string, version int64) error {
	c.mu.Lock()
	c.calls = append(c.calls, cleanupCall{environmentID, projectID, version, key})
	c.mu.Unlock()
	return nil
}

func newImporter(t *testing.T, state *importState, permissions *statePermissions, cache *stateCache) *Logic {
	t.Helper()
	logicLayer, err := New(stateImports{state}, stateProjects{state}, stateConfigs{state}, stateStructures{state}, stateEnums{state}, stateSnapshots{state}, permissions, stateAudits{state}, stateTransactor{state}, cache)
	require.NoError(t, err)
	return logicLayer
}

func importRequest() Request {
	return Request{SourceEnvironmentID: 1, SourceSnapshotID: 12, TargetEnvironmentID: 2, TargetProjectID: 20, Description: "Imported snapshot", Tags: []commonv1.Snapshot_Tag{commonv1.Snapshot_REUSE}, IdempotencyKey: "request-00000001", ConflictStrategy: StrategyFail}
}

func TestImportCreatesMappedTargetAndReplaysOriginalResult(t *testing.T) {
	state := newImportState(t)
	permissions := &statePermissions{denied: map[string]bool{}}
	cache := &stateCache{}
	logicLayer := newImporter(t, state, permissions, cache)

	first, err := logicLayer.Import(context.Background(), permission.Actor{UserID: 9, RequestID: "request"}, importRequest())
	require.NoError(t, err)
	assert.False(t, first.Replayed)
	decoded, err := entity.DecodeConfigSnapshot(first.Snapshot.Content)
	require.NoError(t, err)
	assert.Equal(t, int64(20), decoded.Config.ProjectId)
	assert.NotEqual(t, int64(10), decoded.Config.Id)
	require.Len(t, decoded.Structures, 1)
	require.Len(t, decoded.Enums, 1)
	assert.Equal(t, decoded.Config.Id, decoded.Structures[0].ConfigId)
	assert.Equal(t, decoded.Config.Id, decoded.Enums[0].ConfigId)
	assert.NotEqual(t, int64(101), decoded.Structures[0].Id)
	assert.Equal(t, "feature", decoded.Tree.Value.Key)

	replayed, err := logicLayer.Import(context.Background(), permission.Actor{UserID: 9, RequestID: "retry"}, importRequest())
	require.NoError(t, err)
	assert.True(t, replayed.Replayed)
	assert.Equal(t, first.Snapshot.ID, replayed.Snapshot.ID)
	state.mu.Lock()
	assert.Len(t, state.configs, 1)
	assert.Len(t, state.audits, 1)
	state.mu.Unlock()
	cache.mu.Lock()
	assert.Len(t, cache.calls, 1)
	cache.mu.Unlock()
}

func TestConcurrentImportReplayCreatesOneTarget(t *testing.T) {
	state := newImportState(t)
	logicLayer := newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{})
	const workers = 8
	results := make(chan *Result, workers)
	errorsChannel := make(chan error, workers)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := logicLayer.Import(context.Background(), permission.Actor{UserID: 9}, importRequest())
			results <- result
			errorsChannel <- err
		}()
	}
	group.Wait()
	close(results)
	close(errorsChannel)
	for err := range errorsChannel {
		require.NoError(t, err)
	}
	created := 0
	for result := range results {
		if !result.Replayed {
			created++
		}
	}
	assert.Equal(t, 1, created)
	state.mu.Lock()
	assert.Len(t, state.configs, 1)
	assert.Len(t, state.records, 1)
	state.mu.Unlock()
}

func TestIdempotencyKeyRejectsDifferentRequest(t *testing.T) {
	state := newImportState(t)
	logicLayer := newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{})
	_, err := logicLayer.Import(context.Background(), permission.Actor{UserID: 9}, importRequest())
	require.NoError(t, err)
	changed := importRequest()
	changed.Description = "Different request"
	_, err = logicLayer.Import(context.Background(), permission.Actor{UserID: 9}, changed)
	assert.ErrorIs(t, err, entity.ErrConflict)
}

func TestImportPermissionChecksBothEnvironments(t *testing.T) {
	for _, denied := range []string{"1:" + permission.ConfigRead, "2:" + permission.ConfigWrite} {
		state := newImportState(t)
		permissions := &statePermissions{denied: map[string]bool{denied: true}}
		logicLayer := newImporter(t, state, permissions, &stateCache{})
		_, err := logicLayer.Import(context.Background(), permission.Actor{UserID: 9}, importRequest())
		assert.ErrorIs(t, err, permission.ErrForbidden)
		state.mu.Lock()
		assert.Empty(t, state.records)
		state.mu.Unlock()
	}
}

func TestImportSupportsSameEnvironment(t *testing.T) {
	state := newImportState(t)
	project := state.projects[20]
	project.EnvironmentID = 1
	state.projects[20] = project
	request := importRequest()
	request.TargetEnvironmentID = 1
	result, err := newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{}).Import(context.Background(), permission.Actor{UserID: 9}, request)
	require.NoError(t, err)
	assert.False(t, result.Replayed)
}

func TestReplaceRequiresExplicitTargetAndPreservesTargetKey(t *testing.T) {
	state := newImportState(t)
	state.configs[50] = model.Config{Meta: model.Meta{ID: 50, Version: 3}, ProjectID: 20, Key: "existingFeature", RuntimeVersion: 7}
	request := importRequest()
	request.ConflictStrategy = StrategyReplace
	targetID := int64(50)
	request.TargetConfigID = &targetID
	cache := &stateCache{}
	result, err := newImporter(t, state, &statePermissions{denied: map[string]bool{}}, cache).Import(context.Background(), permission.Actor{UserID: 9}, request)
	require.NoError(t, err)
	decoded, err := entity.DecodeConfigSnapshot(result.Snapshot.Content)
	require.NoError(t, err)
	assert.Equal(t, "existingFeature", decoded.Config.Key)
	assert.Equal(t, uint64(7), decoded.Config.RuntimeVersion)
	state.mu.Lock()
	assert.Equal(t, int64(5), state.configs[50].Version)
	state.mu.Unlock()
	cache.mu.Lock()
	assert.Equal(t, []cleanupCall{{environmentID: 2, projectID: 20, version: 7, key: "existingFeature"}}, cache.calls)
	cache.mu.Unlock()

	request.TargetConfigID = nil
	request.IdempotencyKey = "request-00000002"
	_, err = newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{}).Import(context.Background(), permission.Actor{UserID: 9}, request)
	assert.ErrorIs(t, err, entity.ErrInvalid)
}

func TestAuditFailureRollsBackBusinessAndIdempotencyRecord(t *testing.T) {
	state := newImportState(t)
	state.failAudit = errAudit
	_, err := newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{}).Import(context.Background(), permission.Actor{UserID: 9}, importRequest())
	assert.ErrorIs(t, err, errAudit)
	state.mu.Lock()
	assert.Empty(t, state.configs)
	assert.Empty(t, state.records)
	assert.Len(t, state.snapshots, 1)
	state.mu.Unlock()
}

func TestFailStrategyReturnsConflictForExistingConfigKey(t *testing.T) {
	state := newImportState(t)
	state.configs[50] = model.Config{Meta: model.Meta{ID: 50}, ProjectID: 20, Key: "feature"}
	_, err := newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{}).Import(context.Background(), permission.Actor{UserID: 9}, importRequest())
	assert.ErrorIs(t, err, repository.ErrKeyConflict)
}

func TestReplaceRejectsConfigFromDifferentTargetProject(t *testing.T) {
	state := newImportState(t)
	state.projects[21] = model.Project{Meta: model.Meta{ID: 21}, EnvironmentID: 2, Key: "other-target"}
	state.configs[50] = model.Config{Meta: model.Meta{ID: 50}, ProjectID: 21, Key: "feature"}
	targetID := int64(50)
	request := importRequest()
	request.ConflictStrategy = StrategyReplace
	request.TargetConfigID = &targetID
	_, err := newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{}).Import(context.Background(), permission.Actor{UserID: 9}, request)
	assert.ErrorIs(t, err, base.ErrNotFound)
}

func TestImportRejectsConflictingSourceSchemaBeforeTargetWrites(t *testing.T) {
	state := newImportState(t)
	source := state.snapshots[12]
	decoded, err := entity.DecodeConfigSnapshot(source.Content)
	require.NoError(t, err)
	decoded.Structures = append(decoded.Structures, decoded.Structures[0])
	source.Content, err = entity.EncodeConfigSnapshot(decoded)
	require.NoError(t, err)
	state.snapshots[12] = source
	_, err = newImporter(t, state, &statePermissions{denied: map[string]bool{}}, &stateCache{}).Import(context.Background(), permission.Actor{UserID: 9}, importRequest())
	assert.ErrorIs(t, err, entity.ErrConflict)
	state.mu.Lock()
	assert.Empty(t, state.configs)
	assert.Empty(t, state.records)
	state.mu.Unlock()
}
