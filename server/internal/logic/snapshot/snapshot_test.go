package snapshot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

func TestCreateRejectsInvalidDraftThenSucceedsAfterValueRepair(t *testing.T) {
	typeJSON, err := converter.EncodeFieldType(&commonv1.Field_Type{Kind: &commonv1.Field_Type_StructureKey{StructureKey: "User"}})
	require.NoError(t, err)
	fieldsJSON, err := converter.EncodeFields([]*commonv1.Field{
		{Key: "name", Name: "Name", Type: &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}},
		{Key: "age", Name: "Age", Type: &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_INT}}},
	})
	require.NoError(t, err)
	order := make([]string, 0)
	configs := &configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7, Version: 2}, ProjectID: 3, Key: "user", TypeJSON: typeJSON, Value: `{"name":"Ada"}`}}
	structures := &structureRepositoryFake{order: &order, items: []model.Structure{{Meta: model.Meta{ID: 8, Version: 1}, ConfigID: 7, Key: "User", Name: "User", FieldsJSON: fieldsJSON}}}
	snapshots := &snapshotRepositoryFake{order: &order}
	audits := &auditRepositoryFake{order: &order}
	logicLayer, err := New(configs, structures, &enumRepositoryFake{order: &order}, snapshots, authorizerFake{}, audits, transactorFake{})
	require.NoError(t, err)

	_, err = logicLayer.Create(context.Background(), permission.Actor{UserID: 9, RequestID: "request-1"}, 1, 3, 7, "first snapshot", nil)
	assert.ErrorIs(t, err, entity.ErrInvalid)
	assert.False(t, snapshots.created, "invalid data must never enter a snapshot")

	configs.item.Value = `{"name":"Ada","age":42}`
	order = order[:0]
	created, err := logicLayer.Create(context.Background(), permission.Actor{UserID: 9, RequestID: "request-1"}, 1, 3, 7, "first snapshot", []commonv1.Snapshot_Tag{commonv1.Snapshot_REVIEW})
	require.NoError(t, err)
	assert.Equal(t, int64(11), created.ID)
	assert.Equal(t, []string{"config.lock", "structure.lock", "enum.lock", "snapshot.create", "snapshot.current", "audit", "snapshot.find"}, order)
	decoded, err := entity.DecodeConfigSnapshot(created.Content)
	require.NoError(t, err)
	assert.Equal(t, configs.item.Value, decoded.Config.Value)
	assert.Nil(t, audits.last.DetailsJSON, "audit details must not copy raw config content")
}

func TestLoadRejectsSnapshotFromDifferentConfigBeforeWriting(t *testing.T) {
	order := make([]string, 0)
	snapshots := &snapshotRepositoryFake{order: &order, item: &model.Snapshot{Meta: model.Meta{ID: 11, Version: 1}, ProjectID: 3, ConfigID: 99, Status: model.SnapshotStatusUnreleased, TagsJSON: "[]"}}
	configs := &configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7, Version: 1}, ProjectID: 3}}
	logicLayer, err := New(configs, &structureRepositoryFake{order: &order}, &enumRepositoryFake{order: &order}, snapshots, authorizerFake{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)
	_, err = logicLayer.Load(context.Background(), permission.Actor{UserID: 9}, 1, 7, 11)
	assert.ErrorIs(t, err, base.ErrNotFound)
	assert.Equal(t, []string{"snapshot.find"}, order)
}

func TestWriteOnlyRoleCannotLoadSnapshotContent(t *testing.T) {
	order := make([]string, 0)
	logicLayer, err := New(&configRepositoryFake{order: &order}, &structureRepositoryFake{order: &order}, &enumRepositoryFake{order: &order}, &snapshotRepositoryFake{order: &order}, writeOnlyAuthorizer{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)

	_, err = logicLayer.Load(context.Background(), permission.Actor{UserID: 9}, 1, 7, 11)
	assert.ErrorIs(t, err, permission.ErrForbidden)
	assert.Empty(t, order)
}

func TestDeleteRejectsCurrentSnapshot(t *testing.T) {
	order := make([]string, 0)
	item := &model.Snapshot{Meta: model.Meta{ID: 11, Version: 1}, ProjectID: 3, ConfigID: 7, Status: model.SnapshotStatusUnreleased, IsUsing: true, TagsJSON: "[]"}
	logicLayer, err := New(&configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7}, ProjectID: 3}}, &structureRepositoryFake{order: &order}, &enumRepositoryFake{order: &order}, &snapshotRepositoryFake{order: &order, item: item}, authorizerFake{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)

	err = logicLayer.Delete(context.Background(), permission.Actor{UserID: 9}, 1, 11)
	assert.ErrorIs(t, err, entity.ErrConflict)
	assert.Equal(t, []string{"snapshot.find", "config.lock", "snapshot.lock"}, order)
}

func TestLoadAutoSavesValidDraftWhenCurrentSnapshotIsMissing(t *testing.T) {
	typeJSON, err := converter.EncodeFieldType(&commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}})
	require.NoError(t, err)
	currentConfig := &model.Config{Meta: model.Meta{ID: 7, Version: 2}, ProjectID: 3, Key: "message", TypeJSON: typeJSON, Value: `"draft"`}
	targetContent, err := entity.EncodeConfigSnapshot(&entity.ConfigSnapshot{
		Config: &commonv1.Config{Id: 7, ProjectId: 3, Key: "message", Type: &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}, Value: `"old"`},
		Tree:   &commonv1.TreeNode{Value: &commonv1.Field{Key: "message", Name: "配置值", Type: &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}}},
	})
	require.NoError(t, err)
	order := make([]string, 0)
	snapshots := &snapshotRepositoryFake{order: &order, item: &model.Snapshot{Meta: model.Meta{ID: 11, Version: 1}, ProjectID: 3, ConfigID: 7, ConfigKey: "message", Content: targetContent, Status: model.SnapshotStatusUnreleased, TagsJSON: "[]"}}
	audits := &auditRepositoryFake{order: &order}
	logicLayer, err := New(&configRepositoryFake{order: &order, item: currentConfig}, &structureRepositoryFake{order: &order}, &enumRepositoryFake{order: &order}, snapshots, authorizerFake{}, audits, transactorFake{})
	require.NoError(t, err)

	_, err = logicLayer.Load(context.Background(), permission.Actor{UserID: 9}, 1, 7, 11)
	require.NoError(t, err)
	require.Len(t, snapshots.createdItems, 1)
	autoSaved, err := entity.DecodeConfigSnapshot(snapshots.createdItems[0].Content)
	require.NoError(t, err)
	assert.Equal(t, `"draft"`, autoSaved.Config.Value)
	require.Len(t, audits.items, 2)
	assert.Equal(t, "snapshot.autosave", audits.items[0].Action)
	assert.Equal(t, "snapshot.load", audits.items[1].Action)
	assert.Nil(t, audits.items[0].DetailsJSON)
}

type transactorFake struct{}

func (transactorFake) WithTx(ctx context.Context, fn func(*xorm.Session) error) error { return fn(nil) }

type authorizerFake struct{}

func (authorizerFake) Require(context.Context, int64, int64, ...string) error { return nil }

type writeOnlyAuthorizer struct{}

func (writeOnlyAuthorizer) Require(_ context.Context, _, _ int64, required ...string) error {
	allowed := map[string]struct{}{
		permission.SnapshotWrite: {}, permission.ConfigWrite: {}, permission.StructureWrite: {}, permission.EnumWrite: {},
	}
	for _, key := range required {
		if _, ok := allowed[key]; !ok {
			return permission.ErrForbidden
		}
	}
	return nil
}

type auditRepositoryFake struct {
	order *[]string
	last  *model.AuditLog
	items []*model.AuditLog
}

func (f *auditRepositoryFake) RecordForEnvironmentTx(_ context.Context, _ *xorm.Session, _ int64, item *model.AuditLog) error {
	*f.order = append(*f.order, "audit")
	f.last = item
	f.items = append(f.items, item)
	return nil
}

type configRepositoryFake struct {
	order *[]string
	item  *model.Config
}

func (f *configRepositoryFake) FindByID(context.Context, int64, int64) (*model.Config, error) {
	clone := *f.item
	return &clone, nil
}
func (f *configRepositoryFake) LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.lock")
	clone := *f.item
	return &clone, nil
}
func (f *configRepositoryFake) UpdateTx(context.Context, *xorm.Session, int64, int64, repository.ConfigUpdate) error {
	*f.order = append(*f.order, "config.update")
	return nil
}
func (f *configRepositoryFake) UpdateValueTx(context.Context, *xorm.Session, int64, int64, repository.ConfigValueUpdate) error {
	*f.order = append(*f.order, "config.value.update")
	return nil
}

type structureRepositoryFake struct {
	order *[]string
	items []model.Structure
}

func (f *structureRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error) {
	*f.order = append(*f.order, "structure.lock")
	return f.items, nil
}
func (f *structureRepositoryFake) ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.Structure, int64) error {
	*f.order = append(*f.order, "structure.reconcile")
	return nil
}

type enumRepositoryFake struct {
	order *[]string
	items []model.ConfigEnum
}

func (f *enumRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.lock")
	return f.items, nil
}
func (f *enumRepositoryFake) ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.ConfigEnum, int64) error {
	*f.order = append(*f.order, "enum.reconcile")
	return nil
}

type snapshotRepositoryFake struct {
	order        *[]string
	item         *model.Snapshot
	current      *model.Snapshot
	created      bool
	createdItems []*model.Snapshot
}

func (f *snapshotRepositoryFake) CreateTx(_ context.Context, _ *xorm.Session, _, _, _ int64, item *model.Snapshot) error {
	*f.order = append(*f.order, "snapshot.create")
	f.created = true
	item.ID = int64(11 + len(f.createdItems))
	if f.item != nil {
		item.ID = int64(100 + len(f.createdItems))
	}
	item.Version, item.Status = 0, model.SnapshotStatusUnreleased
	f.createdItems = append(f.createdItems, item)
	if f.item == nil {
		f.item = item
	}
	return nil
}
func (f *snapshotRepositoryFake) FindByID(context.Context, int64, int64) (*model.Snapshot, error) {
	*f.order = append(*f.order, "snapshot.find")
	if f.item == nil {
		return nil, base.ErrNotFound
	}
	clone := *f.item
	return &clone, nil
}
func (f *snapshotRepositoryFake) List(context.Context, int64, repository.SnapshotFilter, base.PageRequest) (base.PageResult[model.Snapshot], error) {
	return base.PageResult[model.Snapshot]{}, nil
}
func (f *snapshotRepositoryFake) FindReleasedForConfig(context.Context, int64, int64) (*model.Snapshot, error) {
	return nil, base.ErrNotFound
}
func (f *snapshotRepositoryFake) FindCurrentForConfig(context.Context, int64, int64) (*model.Snapshot, error) {
	return nil, base.ErrNotFound
}
func (f *snapshotRepositoryFake) FindCurrentForConfigTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error) {
	*f.order = append(*f.order, "snapshot.current.find")
	if f.current == nil {
		return nil, base.ErrNotFound
	}
	clone := *f.current
	return &clone, nil
}
func (f *snapshotRepositoryFake) DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error {
	return nil
}
func (f *snapshotRepositoryFake) LockByID(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error) {
	*f.order = append(*f.order, "snapshot.lock")
	if f.item == nil {
		return nil, base.ErrNotFound
	}
	clone := *f.item
	return &clone, nil
}
func (f *snapshotRepositoryFake) SetCurrent(context.Context, *xorm.Session, int64, int64, int64, int64) error {
	*f.order = append(*f.order, "snapshot.current")
	if f.item != nil {
		f.item.IsUsing = true
	}
	return nil
}
