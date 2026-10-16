package logic

import (
	"context"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type configResources struct {
	configs    repository.ConfigRepository
	structures repository.StructureRepository
	enums      repository.ConfigEnumRepository
}

func (r configResources) lock(ctx context.Context, environmentID, configID int64) (*model.Config, []*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	config, err := r.configs.LockByID(ctx, environmentID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	structureRows, err := r.structures.ListForConfig(ctx, environmentID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	enumRows, err := r.enums.ListForConfig(ctx, environmentID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	structures, err := converter.StructuresToProto(structureRows)
	if err != nil {
		return nil, nil, nil, err
	}
	enums, err := converter.EnumsToProto(enumRows)
	if err != nil {
		return nil, nil, nil, err
	}
	return config, structures, enums, nil
}

func fieldsReferenceEnum(fields []*commonv1.Field, key string) bool {
	for _, field := range fields {
		if field.GetType().GetEnumKey() == key {
			return true
		}
	}
	return false
}

func fieldsReferenceStructure(fields []*commonv1.Field, key string) bool {
	for _, field := range fields {
		if field.GetType().GetStructureKey() == key {
			return true
		}
	}
	return false
}
