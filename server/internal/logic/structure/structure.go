package structure

import (
	"context"
	"fmt"

	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type Repository interface {
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.Structure) error
	FindByID(context.Context, int64, int64) (*model.Structure, error)
	List(context.Context, int64, repository.StructureFilter, base.PageRequest) (base.PageResult[model.Structure], error)
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error)
	UpdateTx(context.Context, *xorm.Session, int64, int64, repository.StructureUpdate) error
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
}
type ConfigRepository interface {
	FindByID(context.Context, int64, int64) (*model.Config, error)
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error)
}
type EnumRepository interface {
	List(context.Context, int64, repository.ConfigEnumFilter, base.PageRequest) (base.PageResult[model.ConfigEnum], error)
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error)
}
type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}
type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

type Logic struct {
	structures   Repository
	configs      ConfigRepository
	enums        EnumRepository
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
}

func New(structures Repository, configs ConfigRepository, enums EnumRepository, permissions Authorizer, audits AuditRepository, transactions database.Transactor) (*Logic, error) {
	if structures == nil || configs == nil || enums == nil || permissions == nil || audits == nil || transactions == nil {
		return nil, fmt.Errorf("structure logic dependencies are incomplete")
	}
	return &Logic{structures: structures, configs: configs, enums: enums, permissions: permissions, audits: audits, transactions: transactions}, nil
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, environmentID, configID int64, key, name, description string) (*model.Structure, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.StructureWrite); err != nil {
		return nil, err
	}
	item := &model.Structure{ConfigID: configID, Key: key, Name: name, Description: description, FieldsJSON: "[]", Meta: model.Meta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID}}
	err := l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		_, structures, enums, err := l.lockedResources(ctx, tx, environmentID, configID)
		if err != nil {
			return err
		}
		structures = append(structures, &commonv1.Structure{Key: key, Name: name, Description: description})
		if _, err := entity.NewSchema(structures, enums); err != nil {
			return err
		}
		if err := l.structures.CreateTx(ctx, tx, environmentID, configID, item); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "structure.create", item.ID))
	})
	if err != nil {
		return nil, err
	}
	return l.structures.FindByID(ctx, environmentID, item.ID)
}

func (l *Logic) Update(ctx context.Context, actor permission.Actor, environmentID, structureID int64, key, name, description string, fields []*commonv1.Field, version int64) (*model.Structure, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.StructureWrite); err != nil {
		return nil, err
	}
	current, err := l.structures.FindByID(ctx, environmentID, structureID)
	if err != nil {
		return nil, err
	}
	fieldsJSON, err := converter.EncodeFields(fields)
	if err != nil {
		return nil, entity.Invalid("invalid structure fields")
	}
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		config, structures, enums, err := l.lockedResources(ctx, tx, environmentID, current.ConfigID)
		if err != nil {
			return err
		}
		lockedCurrent, err := findStructure(structures, structureID)
		if err != nil {
			return err
		}
		referenced, err := referencedStructure(config, structures, lockedCurrent.Key, lockedCurrent.Id)
		if err != nil {
			return err
		}
		if lockedCurrent.Key != key && referenced {
			return entity.Conflict("referenced structure key cannot be changed")
		}
		candidate := &commonv1.Structure{Id: lockedCurrent.Id, ConfigId: lockedCurrent.ConfigId, Key: key, Name: name, Description: description, Fields: fields, Version: uint32(version)}
		for index, item := range structures {
			if item.Id == lockedCurrent.Id {
				structures[index] = candidate
			}
		}
		// Schema edits validate references and cycles only. The draft value may
		// temporarily lag behind until UpdateConfigValue repairs it.
		if _, err := entity.NewSchema(structures, enums); err != nil {
			return err
		}
		if err := l.structures.UpdateTx(ctx, tx, environmentID, structureID, repository.StructureUpdate{Key: key, Name: name, Description: description, FieldsJSON: fieldsJSON, UpdatedBy: actor.UserID, Version: version}); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "structure.update", structureID))
	})
	if err != nil {
		return nil, err
	}
	return l.structures.FindByID(ctx, environmentID, structureID)
}

func (l *Logic) List(ctx context.Context, actor permission.Actor, environmentID, projectID, configID int64, ignoreID *int64) ([]*commonv1.Structure, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.StructureRead); err != nil {
		return nil, err
	}
	config, err := l.configs.FindByID(ctx, environmentID, configID)
	if err != nil {
		return nil, err
	}
	if config.ProjectID != projectID {
		return nil, base.Missing("config")
	}
	items, err := l.allStructures(ctx, environmentID, projectID, configID)
	if err != nil {
		return nil, err
	}
	if ignoreID != nil {
		return entity.FilterStructureChoices(items, *ignoreID)
	}
	return items, nil
}

func (l *Logic) Delete(ctx context.Context, actor permission.Actor, environmentID, structureID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.StructureWrite); err != nil {
		return err
	}
	current, err := l.structures.FindByID(ctx, environmentID, structureID)
	if err != nil {
		return err
	}
	return l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		config, structures, enums, err := l.lockedResources(ctx, tx, environmentID, current.ConfigID)
		if err != nil {
			return err
		}
		lockedCurrent, err := findStructure(structures, structureID)
		if err != nil {
			return err
		}
		referenced, err := referencedStructure(config, structures, lockedCurrent.Key, lockedCurrent.Id)
		if err != nil {
			return err
		}
		if referenced {
			return entity.Conflict("referenced structure cannot be deleted")
		}
		candidate := make([]*commonv1.Structure, 0, len(structures)-1)
		for _, item := range structures {
			if item.Id != lockedCurrent.Id {
				candidate = append(candidate, item)
			}
		}
		if _, err := entity.NewSchema(candidate, enums); err != nil {
			return err
		}
		if err := l.structures.DeleteTx(ctx, tx, environmentID, structureID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "structure.delete", structureID))
	})
}

// lockedResources uses the same order as config and enum writes.
func (l *Logic) lockedResources(ctx context.Context, tx *xorm.Session, environmentID, configID int64) (*model.Config, []*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	config, err := l.configs.LockByID(ctx, tx, environmentID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	structureRows, err := l.structures.ListForConfigTx(ctx, tx, environmentID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	structures := make([]*commonv1.Structure, 0, len(structureRows))
	for index := range structureRows {
		item, err := converter.StructureToProto(&structureRows[index])
		if err != nil {
			return nil, nil, nil, err
		}
		structures = append(structures, item)
	}
	enumRows, err := l.enums.ListForConfigTx(ctx, tx, environmentID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	enums := make([]*commonv1.ConfigEnum, 0, len(enumRows))
	for index := range enumRows {
		item, err := converter.EnumToProto(&enumRows[index])
		if err != nil {
			return nil, nil, nil, err
		}
		enums = append(enums, item)
	}
	return config, structures, enums, nil
}

func findStructure(items []*commonv1.Structure, id int64) (*commonv1.Structure, error) {
	for _, item := range items {
		if item.Id == id {
			return item, nil
		}
	}
	return nil, base.Missing("structure")
}

func (l *Logic) resources(ctx context.Context, environmentID, configID int64) (*model.Config, []*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	config, err := l.configs.FindByID(ctx, environmentID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	structures, err := l.allStructures(ctx, environmentID, config.ProjectID, configID)
	if err != nil {
		return nil, nil, nil, err
	}
	enums := make([]*commonv1.ConfigEnum, 0)
	for offset := 0; ; offset += base.MaxPageSize {
		page, err := l.enums.List(ctx, environmentID, repository.ConfigEnumFilter{ProjectID: config.ProjectID, ConfigID: configID}, base.PageRequest{Offset: offset, Limit: base.MaxPageSize})
		if err != nil {
			return nil, nil, nil, err
		}
		for index := range page.Items {
			converted, err := converter.EnumToProto(&page.Items[index])
			if err != nil {
				return nil, nil, nil, err
			}
			enums = append(enums, converted)
		}
		if len(enums) >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}
	return config, structures, enums, nil
}

func (l *Logic) allStructures(ctx context.Context, environmentID, projectID, configID int64) ([]*commonv1.Structure, error) {
	items := make([]*commonv1.Structure, 0)
	for offset := 0; ; offset += base.MaxPageSize {
		page, err := l.structures.List(ctx, environmentID, repository.StructureFilter{ProjectID: projectID, ConfigID: configID}, base.PageRequest{Offset: offset, Limit: base.MaxPageSize})
		if err != nil {
			return nil, err
		}
		for index := range page.Items {
			converted, err := converter.StructureToProto(&page.Items[index])
			if err != nil {
				return nil, err
			}
			items = append(items, converted)
		}
		if len(items) >= int(page.Total) || len(page.Items) == 0 {
			return items, nil
		}
	}
}

func referencedStructure(config *model.Config, structures []*commonv1.Structure, key string, ignoredID int64) (bool, error) {
	typeValue, err := converter.DecodeFieldType(config.TypeJSON)
	if err != nil {
		return false, err
	}
	if typeValue.GetStructureKey() == key {
		return true, nil
	}
	for _, item := range structures {
		if item.Id == ignoredID {
			continue
		}
		for _, field := range item.Fields {
			if field.GetType().GetStructureKey() == key {
				return true, nil
			}
		}
	}
	return false, nil
}

func audit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{ActorUserID: &actorID, Action: action, ResourceType: "structure", ResourceID: fmt.Sprintf("%d", resourceID), Result: model.AuditResultSucceeded, RequestID: actor.RequestID}
}
