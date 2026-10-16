package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

func TestUpdateValueUsesCanonicalTransactionLockOrder(t *testing.T) {
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	require.NoError(t, err)
	order := make([]string, 0)
	configs := &configRepositoryFake{
		order: &order,
		item: &model.Config{
			Meta: model.Meta{
				ID:      7,
				Version: 3,
			},
			ProjectID: 2,
			Key:       "message",
			TypeJSON:  typeJSON,
			Value:     `"old"`,
		},
	}
	logicLayer := NewConfigLogic(configs, &configStructureRepositoryFake{order: &order}, &configEnumRepositoryFake{order: &order}, configSnapshotRepositoryFake{}, configAuthorizerFake{}, &configAuditRepositoryFake{order: &order}, directTransactions{})

	updated, err := logicLayer.UpdateValue(context.Background(), permission.Actor{UserID: 9}, 1, ConfigValueInput{
		ConfigID: 7,
		Value:    `"new"`,
		Version:  3,
	})
	require.NoError(t, err)
	assert.Equal(t, `"new"`, updated.Value)
	assert.Equal(t, []string{"config.lock", "structure.lock", "enum.lock", "config.value.update", "audit", "config.find"}, order)
}

func TestMetadataUpdateNeverOverwritesConfigValue(t *testing.T) {
	oldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	oldTypeJSON, err := converter.EncodeFieldType(oldType)
	require.NoError(t, err)
	order := make([]string, 0)
	configs := &configRepositoryFake{
		order: &order,
		item: &model.Config{
			Meta: model.Meta{
				ID:      7,
				Version: 3,
			},
			ProjectID: 2,
			Key:       "message",
			TypeJSON:  oldTypeJSON,
			Value:     `"must-survive"`,
		},
	}
	logicLayer := NewConfigLogic(configs, &configStructureRepositoryFake{order: &order}, &configEnumRepositoryFake{order: &order}, configSnapshotRepositoryFake{}, configAuthorizerFake{}, &configAuditRepositoryFake{order: &order}, directTransactions{})

	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, ConfigUpdateInput{
		ConfigID:    7,
		Description: "description",
		FieldType:   &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_INT}},
		IsArray:     false,
		Version:     3,
	})
	require.NoError(t, err)
	assert.Equal(t, `"must-survive"`, configs.item.Value)
	assert.NotContains(t, order, "config.value.update")
}

func TestUpdateValueRevalidatesConcurrentEnumChangeInsideTransaction(t *testing.T) {
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: "Status"}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	require.NoError(t, err)
	valuesJSON, err := converter.EncodeOptions([]*commonv1.SelectOption{{
		Label: "Disabled",
		Value: "DISABLED",
	}})
	require.NoError(t, err)
	order := make([]string, 0)
	configs := &configRepositoryFake{
		order: &order,
		item: &model.Config{
			Meta: model.Meta{
				ID:      7,
				Version: 3,
			},
			ProjectID: 2,
			Key:       "status",
			TypeJSON:  typeJSON,
			Value:     `"ACTIVE"`,
		},
	}
	enums := &configEnumRepositoryFake{
		order: &order,
		items: []model.ConfigEnum{{
			Meta: model.Meta{
				ID:      8,
				Version: 4,
			},
			ConfigID:   7,
			Key:        "Status",
			Name:       "Status",
			ValuesJSON: valuesJSON,
		}},
	}
	logicLayer := NewConfigLogic(configs, &configStructureRepositoryFake{order: &order}, enums, configSnapshotRepositoryFake{}, configAuthorizerFake{}, &configAuditRepositoryFake{order: &order}, directTransactions{})

	_, err = logicLayer.UpdateValue(context.Background(), permission.Actor{UserID: 9}, 1, ConfigValueInput{
		ConfigID: 7,
		Value:    `"ACTIVE"`,
		Version:  3,
	})
	assert.True(t, errors.Is(err, entity.ErrInvalid), "validation must see the enum version locked inside the transaction")
	assert.Equal(t, []string{"config.lock", "structure.lock", "enum.lock"}, order)
}

func TestWriteOnlyRoleCanCreateEmptyConfigButCannotReceiveUpdatedConfig(t *testing.T) {
	order := make([]string, 0)
	configs := &configRepositoryFake{order: &order}
	logicLayer := NewConfigLogic(configs, &configStructureRepositoryFake{order: &order}, &configEnumRepositoryFake{order: &order}, configSnapshotRepositoryFake{}, configWriteOnlyAuthorizer{}, &configAuditRepositoryFake{order: &order}, directTransactions{})

	created, err := logicLayer.Create(context.Background(), permission.Actor{UserID: 9}, 1, ConfigCreateInput{
		ProjectID:   2,
		Key:         "message",
		Description: "",
	})
	require.NoError(t, err, "create returns only the known empty initial value")
	assert.Equal(t, `""`, created.Value)

	order = order[:0]
	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, ConfigUpdateInput{
		ConfigID:    created.ID,
		Description: "",
		FieldType:   &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}},
		IsArray:     false,
		Version:     0,
	})
	assert.ErrorIs(t, err, permission.ErrForbidden)
	assert.Empty(t, order)
	_, err = logicLayer.UpdateValue(context.Background(), permission.Actor{UserID: 9}, 1, ConfigValueInput{
		ConfigID: created.ID,
		Value:    `"secret"`,
		Version:  0,
	})
	assert.ErrorIs(t, err, permission.ErrForbidden)
	assert.Empty(t, order)
}

func TestDeleteRejectsConfigWithAnySnapshot(t *testing.T) {
	order := make([]string, 0)
	configs := &configRepositoryFake{
		order: &order,
		item: &model.Config{
			Meta:      model.Meta{ID: 7},
			ProjectID: 2,
		},
	}
	snapshots := configSnapshotRepositoryFake{
		order: &order,
		any: &model.Snapshot{
			Meta:     model.Meta{ID: 11},
			ConfigID: 7,
			Status:   model.SnapshotStatusUnreleased,
		},
	}
	logicLayer := NewConfigLogic(configs, &configStructureRepositoryFake{order: &order}, &configEnumRepositoryFake{order: &order}, snapshots, configAuthorizerFake{}, &configAuditRepositoryFake{order: &order}, directTransactions{})

	err := logicLayer.Delete(context.Background(), permission.Actor{UserID: 9}, 1, 7)
	assert.ErrorIs(t, err, entity.ErrConflict)
	assert.Equal(t, []string{"config.lock", "snapshot.any"}, order)
}

type configAuthorizerFake struct{}

func (configAuthorizerFake) Require(context.Context, int64, int64, ...string) error { return nil }

type configWriteOnlyAuthorizer struct{}

func (configWriteOnlyAuthorizer) Require(_ context.Context, _, _ int64, required ...string) error {
	for _, key := range required {
		if key != permission.ConfigWrite {
			return permission.ErrForbidden
		}
	}
	return nil
}

type configAuditRepositoryFake struct {
	repository.AuditLogRepository
	order *[]string
}

func (f *configAuditRepositoryFake) RecordForEnvironment(context.Context, int64, *model.AuditLog) error {
	*f.order = append(*f.order, "audit")
	return nil
}

type configRepositoryFake struct {
	order *[]string
	item  *model.Config
}

func (f *configRepositoryFake) Create(_ context.Context, _, _ int64, item *model.Config) error {
	item.ID = 7
	f.item = item
	return nil
}
func (f *configRepositoryFake) FindByID(context.Context, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.find")
	clone := *f.item
	return &clone, nil
}
func (f *configRepositoryFake) List(context.Context, int64, repository.ConfigFilter, repository.PageRequest) (repository.PageResult[model.Config], error) {
	return repository.PageResult[model.Config]{}, nil
}
func (f *configRepositoryFake) Update(_ context.Context, _, _ int64, update repository.ConfigUpdate) error {
	*f.order = append(*f.order, "config.update")
	f.item.Description, f.item.IsArray, f.item.TypeJSON = update.Description, update.IsArray, update.TypeJSON
	f.item.Version++
	return nil
}
func (f *configRepositoryFake) UpdateValue(_ context.Context, _, _ int64, update repository.ConfigValueUpdate) error {
	*f.order = append(*f.order, "config.value.update")
	f.item.Value = update.Value
	f.item.Version++
	return nil
}
func (f *configRepositoryFake) Delete(context.Context, int64, int64, int64) error {
	*f.order = append(*f.order, "config.delete")
	return nil
}
func (f *configRepositoryFake) LockByID(context.Context, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.lock")
	clone := *f.item
	return &clone, nil
}

type configStructureRepositoryFake struct {
	repository.StructureRepository
	order *[]string
}

func (f *configStructureRepositoryFake) List(context.Context, int64, repository.StructureFilter, repository.PageRequest) (repository.PageResult[model.Structure], error) {
	return repository.PageResult[model.Structure]{}, nil
}
func (f *configStructureRepositoryFake) ListForConfig(context.Context, int64, int64) ([]model.Structure, error) {
	*f.order = append(*f.order, "structure.lock")
	return nil, nil
}

type configEnumRepositoryFake struct {
	repository.ConfigEnumRepository
	order *[]string
	items []model.ConfigEnum
}

func (f *configEnumRepositoryFake) List(context.Context, int64, repository.ConfigEnumFilter, repository.PageRequest) (repository.PageResult[model.ConfigEnum], error) {
	return repository.PageResult[model.ConfigEnum]{}, nil
}
func (f *configEnumRepositoryFake) ListForConfig(context.Context, int64, int64) ([]model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.lock")
	return f.items, nil
}

type configSnapshotRepositoryFake struct {
	repository.SnapshotRepository
	order *[]string
	any   *model.Snapshot
}

func (configSnapshotRepositoryFake) FindReleasedForConfig(context.Context, int64, int64) (*model.Snapshot, error) {
	return nil, repository.ErrNotFound
}
func (f configSnapshotRepositoryFake) FindAnyForConfig(context.Context, int64, int64) (*model.Snapshot, error) {
	if f.order != nil {
		*f.order = append(*f.order, "snapshot.any")
	}
	if f.any == nil {
		return nil, repository.ErrNotFound
	}
	return f.any, nil
}
func (configSnapshotRepositoryFake) ListReleasedConfigIDs(context.Context, int64, int64) ([]int64, error) {
	return nil, nil
}
