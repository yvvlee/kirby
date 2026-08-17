package snapshot

import (
	"context"
	"errors"
	"fmt"
	"time"

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

type ConfigRepository interface {
	FindByID(context.Context, int64, int64) (*model.Config, error)
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error)
	UpdateTx(context.Context, *xorm.Session, int64, int64, repository.ConfigUpdate) error
	UpdateValueTx(context.Context, *xorm.Session, int64, int64, repository.ConfigValueUpdate) error
}
type StructureRepository interface {
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error)
	ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.Structure, int64) error
}
type EnumRepository interface {
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error)
	ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.ConfigEnum, int64) error
}
type Repository interface {
	CreateTx(context.Context, *xorm.Session, int64, int64, int64, *model.Snapshot) error
	FindByID(context.Context, int64, int64) (*model.Snapshot, error)
	List(context.Context, int64, repository.SnapshotFilter, base.PageRequest) (base.PageResult[model.Snapshot], error)
	FindReleasedForConfig(context.Context, int64, int64) (*model.Snapshot, error)
	FindCurrentForConfig(context.Context, int64, int64) (*model.Snapshot, error)
	FindCurrentForConfigTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	SetCurrent(context.Context, *xorm.Session, int64, int64, int64, int64) error
}
type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}
type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

type Logic struct {
	configs      ConfigRepository
	structures   StructureRepository
	enums        EnumRepository
	snapshots    Repository
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
}

func New(configs ConfigRepository, structures StructureRepository, enums EnumRepository, snapshots Repository, permissions Authorizer, audits AuditRepository, transactions database.Transactor) (*Logic, error) {
	if configs == nil || structures == nil || enums == nil || snapshots == nil || permissions == nil || audits == nil || transactions == nil {
		return nil, fmt.Errorf("snapshot logic dependencies are incomplete")
	}
	return &Logic{configs: configs, structures: structures, enums: enums, snapshots: snapshots, permissions: permissions, audits: audits, transactions: transactions}, nil
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, environmentID, projectID, configID int64, description string, tags []commonv1.Snapshot_Tag) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotWrite, permission.ConfigRead, permission.StructureRead, permission.EnumRead); err != nil {
		return nil, err
	}
	if err := validateTags(tags); err != nil {
		return nil, err
	}
	tagsJSON, err := converter.EncodeTags(tags)
	if err != nil {
		return nil, entity.Invalid("invalid snapshot tags")
	}
	item := &model.Snapshot{ProjectID: projectID, ConfigID: configID, Description: description, TagsJSON: tagsJSON, Meta: model.Meta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID}}
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		preview, err := l.lockedPreview(ctx, tx, environmentID, configID)
		if err != nil {
			return err
		}
		if preview.Config.ProjectId != projectID {
			return base.Missing("config")
		}
		content, err := entity.EncodeConfigSnapshot(preview)
		if err != nil {
			return err
		}
		item.ConfigKey = preview.Config.Key
		item.Content = content
		if err := l.snapshots.CreateTx(ctx, tx, environmentID, projectID, configID, item); err != nil {
			return err
		}
		if err := l.snapshots.SetCurrent(ctx, tx, environmentID, configID, item.ID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "snapshot.create", item.ID))
	})
	if err != nil {
		return nil, err
	}
	return l.snapshots.FindByID(ctx, environmentID, item.ID)
}

func (l *Logic) Preview(ctx context.Context, actor permission.Actor, environmentID, configID int64) (string, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead, permission.ConfigRead, permission.StructureRead, permission.EnumRead); err != nil {
		return "", err
	}
	var content string
	err := l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		preview, err := l.lockedPreview(ctx, tx, environmentID, configID)
		if err != nil {
			return err
		}
		content, err = entity.EncodeConfigSnapshot(preview)
		return err
	})
	return content, err
}

func (l *Logic) Delete(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotWrite); err != nil {
		return err
	}
	found, err := l.snapshots.FindByID(ctx, environmentID, snapshotID)
	if err != nil {
		return err
	}
	return l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		if _, err := l.configs.LockByID(ctx, tx, environmentID, found.ConfigID); err != nil {
			return err
		}
		locked, err := l.snapshots.LockByID(ctx, tx, environmentID, snapshotID)
		if err != nil {
			return err
		}
		if locked.ConfigID != found.ConfigID {
			return base.Missing("snapshot")
		}
		if locked.IsUsing {
			return entity.Conflict("current snapshot cannot be deleted")
		}
		if locked.Status == model.SnapshotStatusReleased {
			return entity.Conflict("released snapshot cannot be deleted")
		}
		if err := l.snapshots.DeleteTx(ctx, tx, environmentID, snapshotID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "snapshot.delete", snapshotID))
	})
}

func (l *Logic) Get(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return nil, err
	}
	return l.snapshots.FindByID(ctx, environmentID, snapshotID)
}

func (l *Logic) Load(ctx context.Context, actor permission.Actor, environmentID, configID, snapshotID int64) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID,
		permission.SnapshotWrite, permission.ConfigWrite, permission.StructureWrite, permission.EnumWrite,
		permission.SnapshotRead, permission.ConfigRead, permission.StructureRead, permission.EnumRead,
	); err != nil {
		return nil, err
	}
	found, err := l.snapshots.FindByID(ctx, environmentID, snapshotID)
	if err != nil {
		return nil, err
	}
	if found.ConfigID != configID {
		return nil, base.Missing("snapshot")
	}
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		current, currentStructures, currentEnums, err := l.lockedResources(ctx, tx, environmentID, configID)
		if err != nil {
			return err
		}
		currentPreview, err := buildPreview(current, currentStructures, currentEnums)
		if err != nil {
			return err
		}
		currentContent, err := entity.EncodeConfigSnapshot(currentPreview)
		if err != nil {
			return err
		}
		using, err := l.snapshots.FindCurrentForConfigTx(ctx, tx, environmentID, configID)
		if err != nil && !errors.Is(err, base.ErrNotFound) {
			return err
		}
		if errors.Is(err, base.ErrNotFound) || using.Content != currentContent {
			autoSaved := &model.Snapshot{
				ProjectID: current.ProjectID, ConfigID: configID, ConfigKey: current.Key,
				Description: "AutoSave-" + time.Now().UTC().Format("20060102T150405.000000000Z"),
				Content:     currentContent, TagsJSON: "[]",
				Meta: model.Meta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID},
			}
			if err := l.snapshots.CreateTx(ctx, tx, environmentID, current.ProjectID, configID, autoSaved); err != nil {
				return err
			}
			if err := l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "snapshot.autosave", autoSaved.ID)); err != nil {
				return err
			}
		}
		target, err := l.snapshots.LockByID(ctx, tx, environmentID, snapshotID)
		if err != nil {
			return err
		}
		if target.ConfigID != configID || target.ProjectID != current.ProjectID {
			return base.Missing("snapshot")
		}
		content, err := entity.DecodeConfigSnapshot(target.Content)
		if err != nil {
			return err
		}
		if err := validateSnapshotContent(content, current); err != nil {
			return err
		}
		typeJSON, err := converter.EncodeFieldType(content.Config.Type)
		if err != nil {
			return err
		}
		structures, err := snapshotStructures(content.Structures, configID, actor.UserID)
		if err != nil {
			return err
		}
		enums, err := snapshotEnums(content.Enums, configID, actor.UserID)
		if err != nil {
			return err
		}
		if err := l.configs.UpdateTx(ctx, tx, environmentID, configID, repository.ConfigUpdate{Description: content.Config.Description, IsArray: content.Config.IsArray, TypeJSON: typeJSON, UpdatedBy: actor.UserID, Version: current.Version}); err != nil {
			return err
		}
		if err := l.configs.UpdateValueTx(ctx, tx, environmentID, configID, repository.ConfigValueUpdate{Value: content.Config.Value, UpdatedBy: actor.UserID, Version: current.Version + 1}); err != nil {
			return err
		}
		if err := l.structures.ReconcileTx(ctx, tx, environmentID, configID, structures, actor.UserID); err != nil {
			return err
		}
		if err := l.enums.ReconcileTx(ctx, tx, environmentID, configID, enums, actor.UserID); err != nil {
			return err
		}
		if err := l.snapshots.SetCurrent(ctx, tx, environmentID, configID, snapshotID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "snapshot.load", snapshotID))
	})
	if err != nil {
		return nil, err
	}
	return l.snapshots.FindByID(ctx, environmentID, snapshotID)
}

func (l *Logic) Current(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return nil, err
	}
	return l.snapshots.FindCurrentForConfig(ctx, environmentID, configID)
}

func (l *Logic) Released(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return nil, err
	}
	return l.snapshots.FindReleasedForConfig(ctx, environmentID, configID)
}

func (l *Logic) List(ctx context.Context, actor permission.Actor, environmentID, projectID, configID int64, page base.PageRequest) (base.PageResult[model.Snapshot], error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return base.PageResult[model.Snapshot]{}, err
	}
	config, err := l.configs.FindByID(ctx, environmentID, configID)
	if err != nil {
		return base.PageResult[model.Snapshot]{}, err
	}
	if config.ProjectID != projectID {
		return base.PageResult[model.Snapshot]{}, base.Missing("config")
	}
	return l.snapshots.List(ctx, environmentID, repository.SnapshotFilter{ProjectID: projectID, ConfigID: configID}, page)
}

func (l *Logic) lockedPreview(ctx context.Context, tx *xorm.Session, environmentID, configID int64) (*entity.ConfigSnapshot, error) {
	config, structures, enums, err := l.lockedResources(ctx, tx, environmentID, configID)
	if err != nil {
		return nil, err
	}
	return buildPreview(config, structures, enums)
}

func buildPreview(config *model.Config, structures []*commonv1.Structure, enums []*commonv1.ConfigEnum) (*entity.ConfigSnapshot, error) {
	schema, err := entity.NewSchema(structures, enums)
	if err != nil {
		return nil, err
	}
	converted, err := converter.ConfigToProto(config)
	if err != nil {
		return nil, err
	}
	if err := schema.ValidateConfig(converted); err != nil {
		return nil, err
	}
	tree, err := schema.BuildTree(converted)
	if err != nil {
		return nil, err
	}
	return &entity.ConfigSnapshot{Config: converted, Structures: structures, Enums: enums, Tree: tree}, nil
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

func validateSnapshotContent(content *entity.ConfigSnapshot, current *model.Config) error {
	if content == nil || content.Config == nil || content.Config.Type == nil || current == nil {
		return entity.Invalid("snapshot content is incomplete")
	}
	if content.Config.Id != current.ID || content.Config.ProjectId != current.ProjectID || content.Config.Key != current.Key {
		return entity.Conflict("snapshot does not belong to config")
	}
	for _, item := range content.Structures {
		if item == nil || item.ConfigId != current.ID {
			return entity.Invalid("snapshot structure belongs to another config")
		}
	}
	for _, item := range content.Enums {
		if item == nil || item.ConfigId != current.ID {
			return entity.Invalid("snapshot enum belongs to another config")
		}
	}
	schema, err := entity.NewSchema(content.Structures, content.Enums)
	if err != nil {
		return err
	}
	return schema.ValidateConfig(content.Config)
}

func snapshotStructures(items []*commonv1.Structure, configID, actorID int64) ([]*model.Structure, error) {
	result := make([]*model.Structure, 0, len(items))
	for _, item := range items {
		fieldsJSON, err := converter.EncodeFields(item.Fields)
		if err != nil {
			return nil, err
		}
		result = append(result, &model.Structure{ConfigID: configID, Key: item.Key, Name: item.Name, Description: item.Description, FieldsJSON: fieldsJSON, Meta: model.Meta{CreatedBy: actorID, UpdatedBy: actorID}})
	}
	return result, nil
}

func snapshotEnums(items []*commonv1.ConfigEnum, configID, actorID int64) ([]*model.ConfigEnum, error) {
	result := make([]*model.ConfigEnum, 0, len(items))
	for _, item := range items {
		valuesJSON, err := converter.EncodeOptions(item.Values)
		if err != nil {
			return nil, err
		}
		result = append(result, &model.ConfigEnum{ConfigID: configID, Key: item.Key, Name: item.Name, Description: item.Description, ValuesJSON: valuesJSON, Meta: model.Meta{CreatedBy: actorID, UpdatedBy: actorID}})
	}
	return result, nil
}

func validateTags(tags []commonv1.Snapshot_Tag) error {
	seen := make(map[commonv1.Snapshot_Tag]struct{}, len(tags))
	for _, tag := range tags {
		if tag < commonv1.Snapshot_RELEASE || tag > commonv1.Snapshot_REUSE {
			return entity.Invalid("invalid snapshot tag")
		}
		if _, duplicate := seen[tag]; duplicate {
			return entity.Invalid("duplicate snapshot tag")
		}
		seen[tag] = struct{}{}
	}
	return nil
}

func audit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{ActorUserID: &actorID, Action: action, ResourceType: "snapshot", ResourceID: fmt.Sprintf("%d", resourceID), Result: model.AuditResultSucceeded, RequestID: actor.RequestID}
}
