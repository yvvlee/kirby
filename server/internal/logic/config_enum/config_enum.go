package configenum

import (
	"context"
	"fmt"

	"xorm.io/xorm"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/safeint"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type Repository interface {
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.ConfigEnum) error
	FindByID(context.Context, int64, int64) (*model.ConfigEnum, error)
	List(context.Context, int64, repository.ConfigEnumFilter, base.PageRequest) (base.PageResult[model.ConfigEnum], error)
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error)
	UpdateTx(context.Context, *xorm.Session, int64, int64, repository.ConfigEnumUpdate) error
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
}
type ConfigRepository interface {
	FindByID(context.Context, int64, int64) (*model.Config, error)
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error)
}
type StructureRepository interface {
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error)
}
type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}
type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

type Logic struct {
	enums        Repository
	configs      ConfigRepository
	structures   StructureRepository
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
}

func New(enums Repository, configs ConfigRepository, structures StructureRepository, permissions Authorizer, audits AuditRepository, transactions database.Transactor) (*Logic, error) {
	if enums == nil || configs == nil || structures == nil || permissions == nil || audits == nil || transactions == nil {
		return nil, fmt.Errorf("config enum logic dependencies are incomplete")
	}
	return &Logic{enums: enums, configs: configs, structures: structures, permissions: permissions, audits: audits, transactions: transactions}, nil
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, environmentID, configID int64, key, name, description string, values []*commonv1.SelectOption) (*model.ConfigEnum, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnumWrite); err != nil {
		return nil, err
	}
	valuesJSON, err := converter.EncodeOptions(values)
	if err != nil {
		return nil, entity.Invalid("invalid enum values")
	}
	item := &model.ConfigEnum{ConfigID: configID, Key: key, Name: name, Description: description, ValuesJSON: valuesJSON, Meta: model.Meta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID}}
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		_, structures, enums, err := l.lockedResources(ctx, tx, environmentID, configID)
		if err != nil {
			return err
		}
		enums = append(enums, &commonv1.ConfigEnum{Key: key, Name: name, Description: description, Values: values})
		if _, err := entity.NewSchema(structures, enums); err != nil {
			return err
		}
		if err := l.enums.CreateTx(ctx, tx, environmentID, configID, item); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "enum.create", item.ID))
	})
	if err != nil {
		return nil, err
	}
	return l.enums.FindByID(ctx, environmentID, item.ID)
}

func (l *Logic) Update(ctx context.Context, actor permission.Actor, environmentID, enumID int64, key, name, description string, values []*commonv1.SelectOption, version int64) (*model.ConfigEnum, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnumWrite); err != nil {
		return nil, err
	}
	current, err := l.enums.FindByID(ctx, environmentID, enumID)
	if err != nil {
		return nil, err
	}
	valuesJSON, err := converter.EncodeOptions(values)
	if err != nil {
		return nil, entity.Invalid("invalid enum values")
	}
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		config, structures, enums, err := l.lockedResources(ctx, tx, environmentID, current.ConfigID)
		if err != nil {
			return err
		}
		locked, err := findEnum(enums, enumID)
		if err != nil {
			return err
		}
		referenced, err := referencedEnum(config, structures, locked.Key)
		if err != nil {
			return err
		}
		if locked.Key != key && referenced {
			return entity.Conflict("referenced enum key cannot be changed")
		}
		protoVersion, err := safeint.Uint32FromInt64(version)
		if err != nil {
			return entity.Invalid("invalid enum version")
		}
		candidate := &commonv1.ConfigEnum{Id: locked.Id, ConfigId: locked.ConfigId, Key: key, Name: name, Description: description, Values: values, Version: protoVersion}
		for index, item := range enums {
			if item.Id == enumID {
				enums[index] = candidate
			}
		}
		// Draft values are allowed to lag behind a schema edit. Snapshot creation
		// and publishing remain the full schema/value validation gates.
		if _, err := entity.NewSchema(structures, enums); err != nil {
			return err
		}
		if err := l.enums.UpdateTx(ctx, tx, environmentID, enumID, repository.ConfigEnumUpdate{Key: key, Name: name, Description: description, ValuesJSON: valuesJSON, UpdatedBy: actor.UserID, Version: version}); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "enum.update", enumID))
	})
	if err != nil {
		return nil, err
	}
	return l.enums.FindByID(ctx, environmentID, enumID)
}

func (l *Logic) List(ctx context.Context, actor permission.Actor, environmentID, projectID, configID int64) ([]model.ConfigEnum, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnumRead); err != nil {
		return nil, err
	}
	config, err := l.configs.FindByID(ctx, environmentID, configID)
	if err != nil {
		return nil, err
	}
	if config.ProjectID != projectID {
		return nil, base.Missing("config")
	}
	items := make([]model.ConfigEnum, 0)
	for offset := 0; ; offset += base.MaxPageSize {
		page, err := l.enums.List(ctx, environmentID, repository.ConfigEnumFilter{ProjectID: projectID, ConfigID: configID}, base.PageRequest{Offset: offset, Limit: base.MaxPageSize})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if len(items) >= int(page.Total) || len(page.Items) == 0 {
			return items, nil
		}
	}
}

func (l *Logic) Delete(ctx context.Context, actor permission.Actor, environmentID, enumID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnumWrite); err != nil {
		return err
	}
	current, err := l.enums.FindByID(ctx, environmentID, enumID)
	if err != nil {
		return err
	}
	return l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		config, structures, enums, err := l.lockedResources(ctx, tx, environmentID, current.ConfigID)
		if err != nil {
			return err
		}
		locked, err := findEnum(enums, enumID)
		if err != nil {
			return err
		}
		referenced, err := referencedEnum(config, structures, locked.Key)
		if err != nil {
			return err
		}
		if referenced {
			return entity.Conflict("referenced enum cannot be deleted")
		}
		candidate := make([]*commonv1.ConfigEnum, 0, len(enums)-1)
		for _, item := range enums {
			if item.Id != enumID {
				candidate = append(candidate, item)
			}
		}
		if _, err := entity.NewSchema(structures, candidate); err != nil {
			return err
		}
		if err := l.enums.DeleteTx(ctx, tx, environmentID, enumID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "enum.delete", enumID))
	})
}

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

func findEnum(items []*commonv1.ConfigEnum, id int64) (*commonv1.ConfigEnum, error) {
	for _, item := range items {
		if item.Id == id {
			return item, nil
		}
	}
	return nil, base.Missing("config enum")
}

func referencedEnum(config *model.Config, structures []*commonv1.Structure, key string) (bool, error) {
	fieldType, err := converter.DecodeFieldType(config.TypeJSON)
	if err != nil {
		return false, err
	}
	if fieldType.GetEnumKey() == key {
		return true, nil
	}
	for _, structure := range structures {
		for _, field := range structure.Fields {
			if field.GetType().GetEnumKey() == key {
				return true, nil
			}
		}
	}
	return false, nil
}

func audit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{ActorUserID: &actorID, Action: action, ResourceType: "config_enum", ResourceID: fmt.Sprintf("%d", resourceID), Result: model.AuditResultSucceeded, RequestID: actor.RequestID}
}
