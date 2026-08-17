package structure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

func TestUpdateAllowsSchemaFirstValueSecondEvolution(t *testing.T) {
	typeJSON, err := converter.EncodeFieldType(&commonv1.Field_Type{Kind: &commonv1.Field_Type_StructureKey{StructureKey: "User"}})
	require.NoError(t, err)
	oldFields := []*commonv1.Field{{Key: "name", Name: "Name", Type: baseType(commonv1.Field_STRING)}}
	fieldsJSON, err := converter.EncodeFields(oldFields)
	require.NoError(t, err)
	order := make([]string, 0)
	structures := &structureRepositoryFake{order: &order, item: &model.Structure{Meta: model.Meta{ID: 8, Version: 2}, ConfigID: 7, Key: "User", Name: "User", FieldsJSON: fieldsJSON}}
	configs := &configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7, Version: 1}, ProjectID: 3, Key: "user", TypeJSON: typeJSON, Value: `{"name":"Ada"}`}}
	logicLayer, err := New(structures, configs, &enumRepositoryFake{order: &order}, authorizerFake{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)

	newFields := append(oldFields, &commonv1.Field{Key: "age", Name: "Age", Type: baseType(commonv1.Field_INT)})
	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, 8, "User", "User", "", newFields, 2)
	require.NoError(t, err, "adding a required field must not force an impossible simultaneous value update")
	assert.Equal(t, []string{"structure.find", "config.lock", "structure.lock", "enum.lock", "structure.update", "audit", "structure.find"}, order)
}

func baseType(value commonv1.Field_BaseType) *commonv1.Field_Type {
	return &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: value}}
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

func (f *configRepositoryFake) FindByID(context.Context, int64, int64) (*model.Config, error) {
	clone := *f.item
	return &clone, nil
}
func (f *configRepositoryFake) LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.lock")
	clone := *f.item
	return &clone, nil
}

type structureRepositoryFake struct {
	order *[]string
	item  *model.Structure
}

func (f *structureRepositoryFake) CreateTx(context.Context, *xorm.Session, int64, int64, *model.Structure) error {
	return nil
}
func (f *structureRepositoryFake) FindByID(context.Context, int64, int64) (*model.Structure, error) {
	*f.order = append(*f.order, "structure.find")
	clone := *f.item
	return &clone, nil
}
func (f *structureRepositoryFake) List(context.Context, int64, repository.StructureFilter, base.PageRequest) (base.PageResult[model.Structure], error) {
	return base.PageResult[model.Structure]{}, nil
}
func (f *structureRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error) {
	*f.order = append(*f.order, "structure.lock")
	return []model.Structure{*f.item}, nil
}
func (f *structureRepositoryFake) UpdateTx(_ context.Context, _ *xorm.Session, _, _ int64, update repository.StructureUpdate) error {
	*f.order = append(*f.order, "structure.update")
	f.item.Key, f.item.Name, f.item.Description, f.item.FieldsJSON = update.Key, update.Name, update.Description, update.FieldsJSON
	f.item.Version++
	return nil
}
func (f *structureRepositoryFake) DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error {
	return nil
}

type enumRepositoryFake struct{ order *[]string }

func (f *enumRepositoryFake) List(context.Context, int64, repository.ConfigEnumFilter, base.PageRequest) (base.PageResult[model.ConfigEnum], error) {
	return base.PageResult[model.ConfigEnum]{}, nil
}
func (f *enumRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.lock")
	return nil, nil
}
