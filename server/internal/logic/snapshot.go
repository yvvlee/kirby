package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type SnapshotCreateInput struct {
	ProjectID   int64
	ConfigID    int64
	Description string
	Tags        []commonv1.Snapshot_Tag
}

type SnapshotListInput struct {
	ProjectID int64
	ConfigID  int64
	Page      repository.PageRequest
}

type SnapshotLogic struct {
	configs      repository.ConfigRepository
	structures   repository.StructureRepository
	enums        repository.ConfigEnumRepository
	snapshots    repository.SnapshotRepository
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
}

// NewSnapshotLogic creates SnapshotLogic with its configuration dependencies.
//
// Parameters: configs reads and writes configuration rows; structures locks and reconciles structure definitions; enums locks and reconciles enum definitions; snapshots reads and writes snapshot rows; permissions checks the caller permissions; audits records successful changes in the transaction; transactions commits state changes and audit events together.
// Returns: *SnapshotLogic.
// Throws: none.
func NewSnapshotLogic(
	configs repository.ConfigRepository,
	structures repository.StructureRepository,
	enums repository.ConfigEnumRepository,
	snapshots repository.SnapshotRepository,
	permissions environmentPermissionChecker,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
) *SnapshotLogic {
	return &SnapshotLogic{
		configs:      configs,
		structures:   structures,
		enums:        enums,
		snapshots:    snapshots,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
	}
}

// Create creates the resource within the authorized scope.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; input contains the source project and configuration plus snapshot description and tags.
// Returns: the created resource; returns permission, schema-validation, or transactional write failures.
// Throws: none.
func (l *SnapshotLogic) Create(ctx context.Context, actor permission.Actor, environmentID int64, input SnapshotCreateInput) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotWrite, permission.ConfigRead, permission.StructureRead, permission.EnumRead); err != nil {
		return nil, err
	}
	if err := validateTags(input.Tags); err != nil {
		return nil, err
	}
	tagsJSON, err := converter.EncodeTags(input.Tags)
	if err != nil {
		return nil, entity.Invalid("invalid snapshot tags")
	}
	item := &model.Snapshot{
		ProjectID:   input.ProjectID,
		ConfigID:    input.ConfigID,
		Description: input.Description,
		TagsJSON:    tagsJSON,
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		preview, err := l.lockedPreview(txctx, environmentID, input.ConfigID)
		if err != nil {
			return err
		}
		if preview.Config.ProjectId != input.ProjectID {
			return repository.Missing("config")
		}
		content, err := entity.EncodeConfigSnapshot(preview)
		if err != nil {
			return err
		}
		item.ConfigKey = preview.Config.Key
		item.Content = content
		if err := l.snapshots.Create(txctx, environmentID, input.ProjectID, input.ConfigID, item); err != nil {
			return err
		}
		if err := l.snapshots.SetCurrent(txctx, environmentID, input.ConfigID, item.ID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, snapshotAudit(actor, "snapshot.create", item.ID))
	})
	if err != nil {
		return nil, err
	}
	return l.snapshots.FindByID(ctx, environmentID, item.ID)
}

// Preview builds snapshot content from locked configuration resources.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; configID selects the configuration.
// Returns: encoded content from the locked configuration schema and value, or a validation or repository error.
// Throws: none.
func (l *SnapshotLogic) Preview(ctx context.Context, actor permission.Actor, environmentID, configID int64) (string, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead, permission.ConfigRead, permission.StructureRead, permission.EnumRead); err != nil {
		return "", err
	}
	var content string
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		preview, err := l.lockedPreview(txctx, environmentID, configID)
		if err != nil {
			return err
		}
		content, err = entity.EncodeConfigSnapshot(preview)
		return err
	})
	return content, err
}

// Delete removes the resource after checking ownership and dependent state.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; snapshotID selects the snapshot.
// Returns: nil after committing the deletion and audit event; dependency conflicts and permission or write failures are returned.
// Throws: none.
func (l *SnapshotLogic) Delete(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotWrite); err != nil {
		return err
	}
	found, err := l.snapshots.FindByID(ctx, environmentID, snapshotID)
	if err != nil {
		return err
	}
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := l.configs.LockByID(txctx, environmentID, found.ConfigID); err != nil {
			return err
		}
		locked, err := l.snapshots.LockByID(txctx, environmentID, snapshotID)
		if err != nil {
			return err
		}
		if locked.ConfigID != found.ConfigID {
			return repository.Missing("snapshot")
		}
		if locked.IsUsing {
			return entity.Conflict("current snapshot cannot be deleted")
		}
		if locked.Status == model.SnapshotStatusReleased {
			return entity.Conflict("released snapshot cannot be deleted")
		}
		if err := l.snapshots.Delete(txctx, environmentID, snapshotID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, snapshotAudit(actor, "snapshot.delete", snapshotID))
	})
}

// Get returns an authorized snapshot.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; snapshotID selects the snapshot.
// Returns: the authorized snapshot, or a permission or missing-resource error.
// Throws: none.
func (l *SnapshotLogic) Get(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return nil, err
	}
	return l.snapshots.FindByID(ctx, environmentID, snapshotID)
}

// Load preserves the draft before replacing it with the selected snapshot.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; configID selects the configuration; snapshotID selects the snapshot.
// Returns: the selected snapshot after saving the draft and restoring its configuration; all writes roll back together on failure.
// Throws: none.
func (l *SnapshotLogic) Load(ctx context.Context, actor permission.Actor, environmentID, configID, snapshotID int64) (*model.Snapshot, error) {
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
		return nil, repository.Missing("snapshot")
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.loadSnapshot(txctx, actor, environmentID, configID, snapshotID)
	})
	if err != nil {
		return nil, err
	}
	return l.snapshots.FindByID(ctx, environmentID, snapshotID)
}

func (l *SnapshotLogic) loadSnapshot(ctx context.Context, actor permission.Actor, environmentID, configID, snapshotID int64) error {
	current, structures, enums, err := l.lockedResources(ctx, environmentID, configID)
	if err != nil {
		return err
	}
	preview, err := buildPreview(current, structures, enums)
	if err != nil {
		return err
	}
	if err := l.autoSave(ctx, actor, environmentID, current, preview); err != nil {
		return err
	}
	target, err := l.snapshots.LockByID(ctx, environmentID, snapshotID)
	if err != nil {
		return err
	}
	if target.ConfigID != configID || target.ProjectID != current.ProjectID {
		return repository.Missing("snapshot")
	}
	content, err := entity.DecodeConfigSnapshot(target.Content)
	if err != nil {
		return err
	}
	if err := validateSnapshotContent(content, current); err != nil {
		return err
	}
	if err := l.restoreConfig(ctx, actor, environmentID, current, content); err != nil {
		return err
	}
	if err := l.snapshots.SetCurrent(ctx, environmentID, configID, snapshotID, actor.UserID); err != nil {
		return err
	}
	return l.audits.RecordForEnvironment(ctx, environmentID, snapshotAudit(actor, "snapshot.load", snapshotID))
}

func (l *SnapshotLogic) autoSave(ctx context.Context, actor permission.Actor, environmentID int64, current *model.Config, preview *entity.ConfigSnapshot) error {
	content, err := entity.EncodeConfigSnapshot(preview)
	if err != nil {
		return err
	}
	using, err := l.snapshots.FindCurrentForConfig(ctx, environmentID, current.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if err == nil && using.Content == content {
		return nil
	}
	saved := &model.Snapshot{
		ProjectID:   current.ProjectID,
		ConfigID:    current.ID,
		ConfigKey:   current.Key,
		Description: "AutoSave-" + time.Now().UTC().Format("20060102T150405.000000000Z"),
		Content:     content,
		TagsJSON:    "[]",
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	if err := l.snapshots.Create(ctx, environmentID, current.ProjectID, current.ID, saved); err != nil {
		return err
	}
	return l.audits.RecordForEnvironment(ctx, environmentID, snapshotAudit(actor, "snapshot.autosave", saved.ID))
}

func (l *SnapshotLogic) restoreConfig(ctx context.Context, actor permission.Actor, environmentID int64, current *model.Config, content *entity.ConfigSnapshot) error {
	typeJSON, err := converter.EncodeFieldType(content.Config.Type)
	if err != nil {
		return err
	}
	structures, err := snapshotStructures(content.Structures, current.ID, actor.UserID)
	if err != nil {
		return err
	}
	enums, err := snapshotEnums(content.Enums, current.ID, actor.UserID)
	if err != nil {
		return err
	}
	if err := l.configs.Update(ctx, environmentID, current.ID, repository.ConfigUpdate{
		Description: content.Config.Description,
		IsArray:     content.Config.IsArray,
		TypeJSON:    typeJSON,
		UpdatedBy:   actor.UserID,
		Version:     current.Version,
	}); err != nil {
		return err
	}
	if err := l.configs.UpdateValue(ctx, environmentID, current.ID, repository.ConfigValueUpdate{
		Value:     content.Config.Value,
		UpdatedBy: actor.UserID,
		Version:   current.Version + 1,
	}); err != nil {
		return err
	}
	if err := l.structures.Reconcile(ctx, environmentID, current.ID, structures, actor.UserID); err != nil {
		return err
	}
	return l.enums.Reconcile(ctx, environmentID, current.ID, enums, actor.UserID)
}

// Current returns the snapshot currently selected for the configuration.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the current authorized snapshot, or a permission or missing-resource error.
// Throws: none.
func (l *SnapshotLogic) Current(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return nil, err
	}
	return l.snapshots.FindCurrentForConfig(ctx, environmentID, configID)
}

// Released returns the published snapshot for the configuration.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the published authorized snapshot, or a permission or missing-resource error.
// Throws: none.
func (l *SnapshotLogic) Released(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Snapshot, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return nil, err
	}
	return l.snapshots.FindReleasedForConfig(ctx, environmentID, configID)
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; input selects the project, configuration, and requested page.
// Returns: the authorized records, or a permission or repository error.
// Throws: none.
func (l *SnapshotLogic) List(ctx context.Context, actor permission.Actor, environmentID int64, input SnapshotListInput) (repository.PageResult[model.Snapshot], error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.SnapshotRead); err != nil {
		return repository.PageResult[model.Snapshot]{}, err
	}
	config, err := l.configs.FindByID(ctx, environmentID, input.ConfigID)
	if err != nil {
		return repository.PageResult[model.Snapshot]{}, err
	}
	if config.ProjectID != input.ProjectID {
		return repository.PageResult[model.Snapshot]{}, repository.Missing("config")
	}
	return l.snapshots.List(ctx, environmentID, repository.SnapshotFilter{
		ProjectID: input.ProjectID,
		ConfigID:  input.ConfigID,
	}, input.Page)
}

func (l *SnapshotLogic) lockedPreview(ctx context.Context, environmentID, configID int64) (*entity.ConfigSnapshot, error) {
	config, structures, enums, err := l.lockedResources(ctx, environmentID, configID)
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
	return &entity.ConfigSnapshot{
		Config:     converted,
		Structures: structures,
		Enums:      enums,
		Tree:       tree,
	}, nil
}

func (l *SnapshotLogic) lockedResources(ctx context.Context, environmentID, configID int64) (*model.Config, []*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	resources := configResources{
		configs:    l.configs,
		structures: l.structures,
		enums:      l.enums,
	}
	return resources.lock(ctx, environmentID, configID)
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
		result = append(result, &model.Structure{
			ConfigID:    configID,
			Key:         item.Key,
			Name:        item.Name,
			Description: item.Description,
			FieldsJSON:  fieldsJSON,
			Meta: model.Meta{
				CreatedBy: actorID,
				UpdatedBy: actorID,
			},
		})
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
		result = append(result, &model.ConfigEnum{
			ConfigID:    configID,
			Key:         item.Key,
			Name:        item.Name,
			Description: item.Description,
			ValuesJSON:  valuesJSON,
			Meta: model.Meta{
				CreatedBy: actorID,
				UpdatedBy: actorID,
			},
		})
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

func snapshotAudit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: "snapshot",
		ResourceID:   fmt.Sprintf("%d", resourceID),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
