package logic

import (
	"context"
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

func TestUpdateAllowsEnumFirstValueSecondEvolution(t *testing.T) {
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_EnumKey{EnumKey: "Status"}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	require.NoError(t, err)
	oldValues, err := converter.EncodeOptions([]*commonv1.SelectOption{{
		Label: "Active",
		Value: "ACTIVE",
	}})
	require.NoError(t, err)
	order := make([]string, 0)
	enums := &enumRepositoryFake{
		order: &order,
		item: &model.ConfigEnum{
			Meta: model.Meta{
				ID:      8,
				Version: 2,
			},
			ConfigID:   7,
			Key:        "Status",
			Name:       "Status",
			ValuesJSON: oldValues,
		},
	}
	configs := &enumConfigRepositoryFake{
		order: &order,
		item: &model.Config{
			Meta: model.Meta{
				ID:      7,
				Version: 1,
			},
			ProjectID: 3,
			Key:       "status",
			TypeJSON:  typeJSON,
			Value:     `"ACTIVE"`,
		},
	}
	logicLayer := NewEnumLogic(enums, configs, &enumStructureRepositoryFake{order: &order}, enumAuthorizerFake{}, &enumAuditRepositoryFake{order: &order}, directTransactions{})

	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, EnumUpdateInput{
		EnumID:      8,
		Key:         "Status",
		Name:        "Status",
		Description: "",
		Values: []*commonv1.SelectOption{{
			Label: "Disabled",
			Value: "DISABLED",
		}},
		Version: 2,
	})
	require.NoError(t, err, "enum changes must be possible before the draft value is repaired")
	assert.Equal(t, []string{"enum.find", "config.lock", "structure.lock", "enum.lock", "enum.update", "audit", "enum.find"}, order)
	order = order[:0]
	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, EnumUpdateInput{
		EnumID:  8,
		Key:     "StatusRenamed",
		Name:    "Status",
		Version: 3,
	})
	require.ErrorIs(t, err, entity.ErrConflict)
	assert.Equal(t, []string{"enum.find", "config.lock", "structure.lock", "enum.lock"}, order)
	assert.Equal(t, "Status", enums.item.Key)
}

type enumAuthorizerFake struct{}

func (enumAuthorizerFake) Require(context.Context, int64, int64, ...string) error { return nil }

type enumAuditRepositoryFake struct {
	repository.AuditLogRepository
	order *[]string
}

func (f *enumAuditRepositoryFake) RecordForEnvironment(context.Context, int64, *model.AuditLog) error {
	*f.order = append(*f.order, "audit")
	return nil
}

type enumConfigRepositoryFake struct {
	repository.ConfigRepository
	order *[]string
	item  *model.Config
}

func (f *enumConfigRepositoryFake) FindByID(context.Context, int64, int64) (*model.Config, error) {
	clone := *f.item
	return &clone, nil
}
func (f *enumConfigRepositoryFake) LockByID(context.Context, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.lock")
	clone := *f.item
	return &clone, nil
}

type enumStructureRepositoryFake struct {
	repository.StructureRepository
	order *[]string
}

func (f *enumStructureRepositoryFake) ListForConfig(context.Context, int64, int64) ([]model.Structure, error) {
	*f.order = append(*f.order, "structure.lock")
	return nil, nil
}

type enumRepositoryFake struct {
	repository.ConfigEnumRepository
	order *[]string
	item  *model.ConfigEnum
}

func (f *enumRepositoryFake) Create(context.Context, int64, int64, *model.ConfigEnum) error {
	return nil
}
func (f *enumRepositoryFake) FindByID(context.Context, int64, int64) (*model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.find")
	clone := *f.item
	return &clone, nil
}
func (f *enumRepositoryFake) List(context.Context, int64, repository.ConfigEnumFilter, repository.PageRequest) (repository.PageResult[model.ConfigEnum], error) {
	return repository.PageResult[model.ConfigEnum]{}, nil
}
func (f *enumRepositoryFake) ListForConfig(context.Context, int64, int64) ([]model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.lock")
	return []model.ConfigEnum{*f.item}, nil
}
func (f *enumRepositoryFake) Update(_ context.Context, _, _ int64, update repository.ConfigEnumUpdate) error {
	*f.order = append(*f.order, "enum.update")
	f.item.Key, f.item.Name, f.item.Description, f.item.ValuesJSON = update.Key, update.Name, update.Description, update.ValuesJSON
	f.item.Version++
	return nil
}
func (f *enumRepositoryFake) Delete(context.Context, int64, int64, int64) error {
	return nil
}
