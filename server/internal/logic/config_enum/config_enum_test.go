package configenum

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

func TestUpdateAllowsEnumFirstValueSecondEvolution(t *testing.T) {
	typeJSON, err := converter.EncodeFieldType(&commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: "Status"}})
	require.NoError(t, err)
	oldValues, err := converter.EncodeOptions([]*commonv1.SelectOption{{Label: "Active", Value: "ACTIVE"}})
	require.NoError(t, err)
	order := make([]string, 0)
	enums := &enumRepositoryFake{order: &order, item: &model.ConfigEnum{Meta: model.Meta{ID: 8, Version: 2}, ConfigID: 7, Key: "Status", Name: "Status", ValuesJSON: oldValues}}
	configs := &configRepositoryFake{order: &order, item: &model.Config{Meta: model.Meta{ID: 7, Version: 1}, ProjectID: 3, Key: "status", TypeJSON: typeJSON, Value: `"ACTIVE"`}}
	logicLayer, err := New(enums, configs, &structureRepositoryFake{order: &order}, authorizerFake{}, &auditRepositoryFake{order: &order}, transactorFake{})
	require.NoError(t, err)

	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, 8, "Status", "Status", "", []*commonv1.SelectOption{{Label: "Disabled", Value: "DISABLED"}}, 2)
	require.NoError(t, err, "enum changes must be possible before the draft value is repaired")
	assert.Equal(t, []string{"enum.find", "config.lock", "structure.lock", "enum.lock", "enum.update", "audit", "enum.find"}, order)
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

type structureRepositoryFake struct{ order *[]string }

func (f *structureRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error) {
	*f.order = append(*f.order, "structure.lock")
	return nil, nil
}

type enumRepositoryFake struct {
	order *[]string
	item  *model.ConfigEnum
}

func (f *enumRepositoryFake) CreateTx(context.Context, *xorm.Session, int64, int64, *model.ConfigEnum) error {
	return nil
}
func (f *enumRepositoryFake) FindByID(context.Context, int64, int64) (*model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.find")
	clone := *f.item
	return &clone, nil
}
func (f *enumRepositoryFake) List(context.Context, int64, repository.ConfigEnumFilter, base.PageRequest) (base.PageResult[model.ConfigEnum], error) {
	return base.PageResult[model.ConfigEnum]{}, nil
}
func (f *enumRepositoryFake) ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.lock")
	return []model.ConfigEnum{*f.item}, nil
}
func (f *enumRepositoryFake) UpdateTx(_ context.Context, _ *xorm.Session, _, _ int64, update repository.ConfigEnumUpdate) error {
	*f.order = append(*f.order, "enum.update")
	f.item.Key, f.item.Name, f.item.Description, f.item.ValuesJSON = update.Key, update.Name, update.Description, update.ValuesJSON
	f.item.Version++
	return nil
}
func (f *enumRepositoryFake) DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error {
	return nil
}
