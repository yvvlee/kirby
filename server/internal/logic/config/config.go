package config

import (
	"context"
	"errors"
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
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.Config) error
	FindByID(context.Context, int64, int64) (*model.Config, error)
	List(context.Context, int64, repository.ConfigFilter, base.PageRequest) (base.PageResult[model.Config], error)
	UpdateTx(context.Context, *xorm.Session, int64, int64, repository.ConfigUpdate) error
	UpdateValueTx(context.Context, *xorm.Session, int64, int64, repository.ConfigValueUpdate) error
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error)
}

type StructureRepository interface {
	List(context.Context, int64, repository.StructureFilter, base.PageRequest) (base.PageResult[model.Structure], error)
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error)
}

type EnumRepository interface {
	List(context.Context, int64, repository.ConfigEnumFilter, base.PageRequest) (base.PageResult[model.ConfigEnum], error)
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error)
}

type SnapshotRepository interface {
	FindReleasedForConfig(context.Context, int64, int64) (*model.Snapshot, error)
	FindAnyForConfigTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	ListReleasedConfigIDs(context.Context, int64, int64) ([]int64, error)
}

type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}
type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

type Logic struct {
	configs      Repository
	structures   StructureRepository
	enums        EnumRepository
	snapshots    SnapshotRepository
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
}

func New(configs Repository, structures StructureRepository, enums EnumRepository, snapshots SnapshotRepository, permissions Authorizer, audits AuditRepository, transactions database.Transactor) (*Logic, error) {
	if configs == nil || structures == nil || enums == nil || snapshots == nil || permissions == nil || audits == nil || transactions == nil {
		return nil, fmt.Errorf("config logic dependencies are incomplete")
	}
	return &Logic{configs: configs, structures: structures, enums: enums, snapshots: snapshots, permissions: permissions, audits: audits, transactions: transactions}, nil
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, environmentID, projectID int64, key, description string) (*model.Config, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite); err != nil {
		return nil, err
	}
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	if err != nil {
		return nil, err
	}
	item := &model.Config{ProjectID: projectID, Key: key, Description: description, TypeJSON: typeJSON, Value: `""`, Meta: model.Meta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID}}
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		if err := l.configs.CreateTx(ctx, tx, environmentID, projectID, item); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "config.create", item.ID))
	})
	if err != nil {
		return nil, err
	}
	// A write-only role may create a config because the returned value is the
	// known empty initial value. Do not re-read after commit and accidentally
	// expose a value changed concurrently by a reader/editor.
	return item, nil
}

func (l *Logic) Update(ctx context.Context, actor permission.Actor, environmentID, configID int64, description string, fieldType *commonv1.Field_Type, isArray bool, version int64) (*model.Config, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite, permission.ConfigRead); err != nil {
		return nil, err
	}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	if err != nil {
		return nil, entity.Invalid("invalid config type")
	}
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		_, structures, enums, err := l.lockedResources(ctx, tx, environmentID, configID)
		if err != nil {
			return err
		}
		schema, err := entity.NewSchema(structures, enums)
		if err != nil {
			return err
		}
		// Type changes may intentionally make the current draft value invalid.
		// UpdateValue and snapshot creation are the explicit validation gates.
		if _, err := schema.DefaultValue(fieldType, isArray); err != nil {
			return err
		}
		if err := l.configs.UpdateTx(ctx, tx, environmentID, configID, repository.ConfigUpdate{Description: description, IsArray: isArray, TypeJSON: typeJSON, UpdatedBy: actor.UserID, Version: version}); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "config.update", configID))
	})
	if err != nil {
		return nil, err
	}
	return l.configs.FindByID(ctx, environmentID, configID)
}

func (l *Logic) UpdateValue(ctx context.Context, actor permission.Actor, environmentID, configID int64, value string, version int64) (*model.Config, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite, permission.ConfigRead); err != nil {
		return nil, err
	}
	err := l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		current, structures, enums, err := l.lockedResources(ctx, tx, environmentID, configID)
		if err != nil {
			return err
		}
		schema, err := entity.NewSchema(structures, enums)
		if err != nil {
			return err
		}
		candidate, err := converter.ConfigToProto(current)
		if err != nil {
			return err
		}
		candidate.Value = value
		if err := schema.ValidateConfig(candidate); err != nil {
			return err
		}
		if err := l.configs.UpdateValueTx(ctx, tx, environmentID, configID, repository.ConfigValueUpdate{Value: value, UpdatedBy: actor.UserID, Version: version}); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "config.value.update", configID))
	})
	if err != nil {
		return nil, err
	}
	return l.configs.FindByID(ctx, environmentID, configID)
}

func (l *Logic) List(ctx context.Context, actor permission.Actor, environmentID int64, filter repository.ConfigFilter) ([]model.Config, map[int64]bool, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigRead); err != nil {
		return nil, nil, err
	}
	items := make([]model.Config, 0)
	for offset := 0; ; offset += base.MaxPageSize {
		page, err := l.configs.List(ctx, environmentID, filter, base.PageRequest{Offset: offset, Limit: base.MaxPageSize})
		if err != nil {
			return nil, nil, err
		}
		items = append(items, page.Items...)
		if len(items) >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}
	released := make(map[int64]bool)
	projects := make(map[int64]struct{})
	for _, item := range items {
		projects[item.ProjectID] = struct{}{}
	}
	for projectID := range projects {
		ids, err := l.snapshots.ListReleasedConfigIDs(ctx, environmentID, projectID)
		if err != nil {
			return nil, nil, err
		}
		for _, id := range ids {
			released[id] = true
		}
	}
	return items, released, nil
}

func (l *Logic) Delete(ctx context.Context, actor permission.Actor, environmentID, configID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite); err != nil {
		return err
	}
	return l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		if _, err := l.configs.LockByID(ctx, tx, environmentID, configID); err != nil {
			return err
		}
		if _, err := l.snapshots.FindAnyForConfigTx(ctx, tx, environmentID, configID); err == nil {
			return entity.Conflict("config with snapshots cannot be deleted")
		} else if !errors.Is(err, base.ErrNotFound) {
			return err
		}
		if err := l.configs.DeleteTx(ctx, tx, environmentID, configID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "config.delete", configID))
	})
}

// lockedResources is the canonical lock order for configuration-domain writes:
// config, structures, then enums. Validation must use only these locked rows.
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

func (l *Logic) Detail(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Config, *commonv1.TreeNode, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigRead, permission.StructureRead, permission.EnumRead); err != nil {
		return nil, nil, err
	}
	current, err := l.configs.FindByID(ctx, environmentID, configID)
	if err != nil {
		return nil, nil, err
	}
	structures, enums, err := l.resources(ctx, environmentID, current.ProjectID, configID)
	if err != nil {
		return nil, nil, err
	}
	schema, err := entity.NewSchema(structures, enums)
	if err != nil {
		return nil, nil, err
	}
	converted, err := converter.ConfigToProto(current)
	if err != nil {
		return nil, nil, err
	}
	tree, err := schema.BuildTree(converted)
	if err != nil {
		return nil, nil, err
	}
	return current, tree, nil
}

func (l *Logic) resources(ctx context.Context, environmentID, projectID, configID int64) ([]*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	structures := make([]*commonv1.Structure, 0)
	for offset := 0; ; offset += base.MaxPageSize {
		page, err := l.structures.List(ctx, environmentID, repository.StructureFilter{ProjectID: projectID, ConfigID: configID}, base.PageRequest{Offset: offset, Limit: base.MaxPageSize})
		if err != nil {
			return nil, nil, err
		}
		for index := range page.Items {
			item, err := converter.StructureToProto(&page.Items[index])
			if err != nil {
				return nil, nil, err
			}
			structures = append(structures, item)
		}
		if len(structures) >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}
	enums := make([]*commonv1.ConfigEnum, 0)
	for offset := 0; ; offset += base.MaxPageSize {
		page, err := l.enums.List(ctx, environmentID, repository.ConfigEnumFilter{ProjectID: projectID, ConfigID: configID}, base.PageRequest{Offset: offset, Limit: base.MaxPageSize})
		if err != nil {
			return nil, nil, err
		}
		for index := range page.Items {
			item, err := converter.EnumToProto(&page.Items[index])
			if err != nil {
				return nil, nil, err
			}
			enums = append(enums, item)
		}
		if len(enums) >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}
	return structures, enums, nil
}

func audit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{ActorUserID: &actorID, Action: action, ResourceType: "config", ResourceID: fmt.Sprintf("%d", resourceID), Result: model.AuditResultSucceeded, RequestID: actor.RequestID}
}
