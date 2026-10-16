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

func TestUpdateAllowsSchemaFirstValueSecondEvolution(t *testing.T) {
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_StructureKey{StructureKey: "User"}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	require.NoError(t, err)
	oldFields := []*commonv1.Field{{
		Key:  "name",
		Name: "Name",
		Type: baseType(commonv1.Field_STRING),
	}}
	fieldsJSON, err := converter.EncodeFields(oldFields)
	require.NoError(t, err)
	order := make([]string, 0)
	structures := &structureRepositoryFake{
		order: &order,
		item: &model.Structure{
			Meta: model.Meta{
				ID:      8,
				Version: 2,
			},
			ConfigID:   7,
			Key:        "User",
			Name:       "User",
			FieldsJSON: fieldsJSON,
		},
	}
	configs := &structureConfigRepositoryFake{
		order: &order,
		item: &model.Config{
			Meta: model.Meta{
				ID:      7,
				Version: 1,
			},
			ProjectID: 3,
			Key:       "user",
			TypeJSON:  typeJSON,
			Value:     `{"name":"Ada"}`,
		},
	}
	logicLayer := NewStructureLogic(structures, configs, &structureEnumRepositoryFake{order: &order}, structureAuthorizerFake{}, &structureAuditRepositoryFake{order: &order}, directTransactions{})

	newFields := append(oldFields, &commonv1.Field{
		Key:  "age",
		Name: "Age",
		Type: baseType(commonv1.Field_INT),
	})
	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, StructureUpdateInput{
		StructureID: 8,
		Key:         "User",
		Name:        "User",
		Description: "",
		Fields:      newFields,
		Version:     2,
	})
	require.NoError(t, err, "adding a required field must not force an impossible simultaneous value update")
	assert.Equal(t, []string{"structure.find", "config.lock", "structure.lock", "enum.lock", "structure.update", "audit", "structure.find"}, order)
	order = order[:0]
	_, err = logicLayer.Update(context.Background(), permission.Actor{UserID: 9}, 1, StructureUpdateInput{
		StructureID: 8,
		Key:         "UserRenamed",
		Name:        "User",
		Version:     3,
	})
	require.ErrorIs(t, err, entity.ErrConflict)
	assert.Equal(t, []string{"structure.find", "config.lock", "structure.lock", "enum.lock"}, order)
	assert.Equal(t, "User", structures.item.Key)
}

func baseType(value commonv1.Field_BaseType) *commonv1.Field_Type {
	return &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: value}}
}

type structureAuthorizerFake struct{}

func (structureAuthorizerFake) Require(context.Context, int64, int64, ...string) error { return nil }

type structureAuditRepositoryFake struct {
	repository.AuditLogRepository
	order *[]string
}

func (f *structureAuditRepositoryFake) RecordForEnvironment(context.Context, int64, *model.AuditLog) error {
	*f.order = append(*f.order, "audit")
	return nil
}

type structureConfigRepositoryFake struct {
	repository.ConfigRepository
	order *[]string
	item  *model.Config
}

func (f *structureConfigRepositoryFake) FindByID(context.Context, int64, int64) (*model.Config, error) {
	clone := *f.item
	return &clone, nil
}
func (f *structureConfigRepositoryFake) LockByID(context.Context, int64, int64) (*model.Config, error) {
	*f.order = append(*f.order, "config.lock")
	clone := *f.item
	return &clone, nil
}

type structureRepositoryFake struct {
	repository.StructureRepository
	order *[]string
	item  *model.Structure
}

func (f *structureRepositoryFake) Create(context.Context, int64, int64, *model.Structure) error {
	return nil
}
func (f *structureRepositoryFake) FindByID(context.Context, int64, int64) (*model.Structure, error) {
	*f.order = append(*f.order, "structure.find")
	clone := *f.item
	return &clone, nil
}
func (f *structureRepositoryFake) List(context.Context, int64, repository.StructureFilter, repository.PageRequest) (repository.PageResult[model.Structure], error) {
	return repository.PageResult[model.Structure]{}, nil
}
func (f *structureRepositoryFake) ListForConfig(context.Context, int64, int64) ([]model.Structure, error) {
	*f.order = append(*f.order, "structure.lock")
	return []model.Structure{*f.item}, nil
}
func (f *structureRepositoryFake) Update(_ context.Context, _, _ int64, update repository.StructureUpdate) error {
	*f.order = append(*f.order, "structure.update")
	f.item.Key, f.item.Name, f.item.Description, f.item.FieldsJSON = update.Key, update.Name, update.Description, update.FieldsJSON
	f.item.Version++
	return nil
}
func (f *structureRepositoryFake) Delete(context.Context, int64, int64, int64) error {
	return nil
}

type structureEnumRepositoryFake struct {
	repository.ConfigEnumRepository
	order *[]string
}

func (f *structureEnumRepositoryFake) List(context.Context, int64, repository.ConfigEnumFilter, repository.PageRequest) (repository.PageResult[model.ConfigEnum], error) {
	return repository.PageResult[model.ConfigEnum]{}, nil
}
func (f *structureEnumRepositoryFake) ListForConfig(context.Context, int64, int64) ([]model.ConfigEnum, error) {
	*f.order = append(*f.order, "enum.lock")
	return nil, nil
}
