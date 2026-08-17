package config

import (
	"context"
	"errors"
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

func TestUpdateValueUsesCanonicalTransactionLockOrder(t *testing.T) {
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	require.NoError(t, err)
	order := make([]string, 0)
	configs := &configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7, Version: 3}, ProjectID: 2, Key: "message", TypeJSON: typeJSON, Value: `"old"`}}
	logicLayer, err := New(configs, &structureRepositoryFake{order: &order}, &enumRepositoryFake{order: &order}, snapshotRepositoryFake{}, authorizerFake{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)

	updated, err := logicLayer.UpdateValue(context.Background(), permission.Actor{UserID: 9}, 1, 7, `"new"`, 3)
	require.NoError(t, err)
	assert.Equal(t, `"new"`, updated.Value)
	assert.Equal(t, []string{"config.lock", "structure.lock", "enum.lock", "config.value.update", "audit", "config.find"}, order)
}

func TestMetadataUpdateNeverOverwritesConfigValue(t *testing.T) {
	oldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	oldTypeJSON, err := converter.EncodeFieldType(oldType)
	require.NoError(t, err)
	order := make([]string, 0)
	configs := &configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7, Version: 3}, ProjectID: 2, Key: "message", TypeJSON: oldTypeJSON, Value: `"must-survive"`}}
	logicLayer, err := New(configs, &structureRepositoryFake{order: &order}, &enumRepositoryFake{order: &order}, snapshotRepositoryFake{}, authorizerFake{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)

	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, 7, "description", &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_INT}}, false, 3)
	require.NoError(t, err)
	assert.Equal(t, `"must-survive"`, configs.item.Value)
	assert.NotContains(t, order, "config.value.update")
}

func TestUpdateValueRevalidatesConcurrentEnumChangeInsideTransaction(t *testing.T) {
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: "Status"}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	require.NoError(t, err)
	valuesJSON, err := converter.EncodeOptions([]*commonv1.SelectOption{{Label: "Disabled", Value: "DISABLED"}})
	require.NoError(t, err)
	order := make([]string, 0)
	configs := &configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7, Version: 3}, ProjectID: 2, Key: "status", TypeJSON: typeJSON, Value: `"ACTIVE"`}}
	enums := &enumRepositoryFake{order: &order, items: []model.ConfigEnum{{Meta: model.Meta{ID: 8, Version: 4}, ConfigID: 7, Key: "Status", Name: "Status", ValuesJSON: valuesJSON}}}
	logicLayer, err := New(configs, &structureRepositoryFake{order: &order}, enums, snapshotRepositoryFake{}, authorizerFake{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)

	_, err = logicLayer.UpdateValue(context.Background(), permission.Actor{UserID: 9}, 1, 7, `"ACTIVE"`, 3)
	assert.True(t, errors.Is(err, entity.ErrInvalid), "validation must see the enum version locked inside the transaction")
	assert.Equal(t, []string{"config.lock", "structure.lock", "enum.lock"}, order)
}

type transactorFake struct{}

func (transactorFake) WithTx(ctx context.Context, fn func(*xorm.Session) error) error { return fn(nil) }

type authorizerFake struct{}

func (authorizerFake) Require(context.Context, int64, int64, ...string) error { return nil }

type auditRepositoryFake struct{ order *[]string }

func (f *auditRepositoryFake) RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error {
	*f.order = append(*f.order, "audit")
	return nil
}

type configRepositoryFake struct {
	order *[]string
	item  *model.Config
}

func (f *configRepositoryFake) CreateTx(context.Context, *xorm.Session, int64, int64, *model.Config) error {
	return nil
}
func (f *configRepositoryFake) FindByID(context.Context, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.find")
	clone := *f.item
	return &clone, nil
}
func (f *configRepositoryFake) List(context.Context, int64, repository.ConfigFilter, base.PageRequest) (base.PageResult[model.Config], error) {
	return base.PageResult[model.Config]{}, nil
}
func (f *configRepositoryFake) UpdateTx(_ context.Context, _ *xorm.Session, _, _ int64, update repository.ConfigUpdate) error {
	*f.order = append(*f.order, "config.update")
	f.item.Description, f.item.IsArray, f.item.TypeJSON = update.Description, update.IsArray, update.TypeJSON
	f.item.Version++
	return nil
}
func (f *configRepositoryFake) UpdateValueTx(_ context.Context, _ *xorm.Session, _, _ int64, update repository.ConfigValueUpdate) error {
	*f.order = append(*f.order, "config.value.update")
	f.item.Value = update.Value
	f.item.Version++
	return nil
}
func (f *configRepositoryFake) DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error {
	return nil
}
func (f *configRepositoryFake) LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.lock")
	clone := *f.item
	return &clone, nil
}

type structureRepositoryFake struct{ order *[]string }

func (f *structureRepositoryFake) List(context.Context, int64, repository.StructureFilter, base.PageRequest) (base.PageResult[model.Structure], error) {
	return base.PageResult[model.Structure]{}, nil
}
func (f *structureRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error) {
	*f.order = append(*f.order, "structure.lock")
	return nil, nil
}

type enumRepositoryFake struct {
	order *[]string
	items []model.ConfigEnum
}

func (f *enumRepositoryFake) List(context.Context, int64, repository.ConfigEnumFilter, base.PageRequest) (base.PageResult[model.ConfigEnum], error) {
	return base.PageResult[model.ConfigEnum]{}, nil
}
func (f *enumRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.lock")
	return f.items, nil
}

type snapshotRepositoryFake struct{}

func (snapshotRepositoryFake) FindReleasedForConfig(context.Context, int64, int64) (*model.Snapshot, error) {
	return nil, base.ErrNotFound
}
func (snapshotRepositoryFake) FindReleasedForConfigTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error) {
	return nil, base.ErrNotFound
}
func (snapshotRepositoryFake) ListReleasedConfigIDs(context.Context, int64, int64) ([]int64, error) {
	return nil, nil
}
