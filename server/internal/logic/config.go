package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/samber/lo"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type ConfigLogic struct {
	configs      repository.ConfigRepository
	structures   repository.StructureRepository
	enums        repository.ConfigEnumRepository
	snapshots    repository.SnapshotRepository
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
}

// NewConfigLogic creates ConfigLogic with its configuration dependencies.
//
// Parameters: configs reads and writes configuration rows; structures locks and reconciles structure definitions; enums locks and reconciles enum definitions; snapshots reads and writes snapshot rows; permissions checks the caller permissions; audits records successful changes in the transaction; transactions commits state changes and audit events together.
// Returns: *ConfigLogic.
// Throws: none.
func NewConfigLogic(
	configs repository.ConfigRepository,
	structures repository.StructureRepository,
	enums repository.ConfigEnumRepository,
	snapshots repository.SnapshotRepository,
	permissions environmentPermissionChecker,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
) *ConfigLogic {
	return &ConfigLogic{
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
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, and the project ID, new configuration key, and description.
// Returns: the created resource; returns permission, schema-validation, or transactional write failures.
// Throws: none.
func (l *ConfigLogic) Create(ctx context.Context, param *entity.CreateConfigParam) (*commonv1.Config, error) {
	actor, environmentID := param.Actor, param.EnvironmentID
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite); err != nil {
		return nil, err
	}
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	if err != nil {
		return nil, err
	}
	item := &model.Config{
		ProjectID:   param.ProjectID,
		Key:         param.Key,
		Description: param.Description,
		TypeJSON:    typeJSON,
		Value:       `""`,
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := l.configs.Create(txctx, environmentID, param.ProjectID, item); err != nil {
			return err
		}
		_, err := l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.create", item.ID))
		return err
	})
	if err != nil {
		return nil, err
	}
	// A write-only role may create a config because the returned value is the
	// known empty initial value. Do not re-read after commit and accidentally
	// expose a value changed concurrently by a reader/editor.
	return converter.ConfigToProto(item)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, the configuration ID, replacement schema metadata, and expected version.
// Returns: the reloaded resource after commit; returns permission, schema, dependency-conflict, or stale-version errors.
// Throws: none.
func (l *ConfigLogic) Update(ctx context.Context, param *entity.UpdateConfigParam) (*commonv1.Config, error) {
	actor, environmentID := param.Actor, param.EnvironmentID
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite, permission.ConfigRead); err != nil {
		return nil, err
	}
	typeJSON, err := converter.EncodeFieldType(param.FieldType)
	if err != nil {
		return nil, entity.Invalid("invalid config type")
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		_, structures, enums, err := l.lockedResources(txctx, environmentID, param.ConfigID)
		if err != nil {
			return err
		}
		schema, err := entity.NewSchema(structures, enums)
		if err != nil {
			return err
		}
		// Type changes may intentionally make the current draft value invalid.
		// UpdateValue and snapshot creation are the explicit validation gates.
		if _, err := schema.DefaultValue(param.FieldType, param.IsArray); err != nil {
			return err
		}
		if _, err := l.configs.Update(txctx, environmentID, param.ConfigID, repository.ConfigUpdate{
			Description: param.Description,
			IsArray:     param.IsArray,
			TypeJSON:    typeJSON,
			UpdatedBy:   actor.UserID,
			Version:     param.Version,
		}); err != nil {
			return err
		}
		_, err = l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.update", param.ConfigID))
		return err
	})
	if err != nil {
		return nil, err
	}
	updated, err := l.configs.FindByID(ctx, environmentID, param.ConfigID)
	if err != nil {
		return nil, err
	}
	return converter.ConfigToProto(updated)
}

// UpdateValue validates and updates the draft configuration value.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, the configuration ID, replacement JSON value, and expected version.
// Returns: the reloaded configuration after validation and commit, or a permission, value-validation, or stale-version error.
// Throws: none.
func (l *ConfigLogic) UpdateValue(ctx context.Context, param *entity.UpdateConfigValueParam) (*commonv1.Config, error) {
	actor, environmentID := param.Actor, param.EnvironmentID
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite, permission.ConfigRead); err != nil {
		return nil, err
	}
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		current, structures, enums, err := l.lockedResources(txctx, environmentID, param.ConfigID)
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
		candidate.Value = param.Value
		if err := schema.ValidateConfig(candidate); err != nil {
			return err
		}
		if _, err := l.configs.UpdateValue(txctx, environmentID, param.ConfigID, repository.ConfigValueUpdate{
			Value:     param.Value,
			UpdatedBy: actor.UserID,
			Version:   param.Version,
		}); err != nil {
			return err
		}
		_, err = l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.value.update", param.ConfigID))
		return err
	})
	if err != nil {
		return nil, err
	}
	updated, err := l.configs.FindByID(ctx, environmentID, param.ConfigID)
	if err != nil {
		return nil, err
	}
	return converter.ConfigToProto(updated)
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, and the filter restricting the returned rows.
// Returns: the authorized records with their release markers, or a permission, repository, or conversion error.
// Throws: none.
func (l *ConfigLogic) List(ctx context.Context, param *entity.ListConfigParam) ([]*adminv1.ListConfigItem, error) {
	environmentID := param.EnvironmentID
	if err := l.permissions.Require(ctx, param.Actor.UserID, environmentID, permission.ConfigRead); err != nil {
		return nil, err
	}
	items := make([]model.Config, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.configs.List(ctx, environmentID, param.Filter, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if len(items) >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}
	released := make(map[int64]bool)
	projectIDs := lo.Uniq(lo.Map(items, func(item model.Config, _ int) int64 { return item.ProjectID }))
	for _, projectID := range projectIDs {
		ids, err := l.snapshots.ListReleasedConfigIDs(ctx, environmentID, projectID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			released[id] = true
		}
	}
	result := make([]*adminv1.ListConfigItem, 0, len(items))
	for index := range items {
		converted, err := converter.ConfigToProto(&items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, &adminv1.ListConfigItem{
			Config:     converted,
			IsReleased: released[items[index].ID],
		})
	}
	return result, nil
}

// Delete removes the resource after checking ownership and dependent state.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; configID selects the configuration.
// Returns: nil after committing the deletion and audit event; dependency conflicts and permission or write failures are returned.
// Throws: none.
func (l *ConfigLogic) Delete(ctx context.Context, actor permission.Actor, environmentID, configID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite); err != nil {
		return err
	}
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := l.configs.LockByID(txctx, environmentID, configID); err != nil {
			return err
		}
		if _, err := l.snapshots.FindAnyForConfig(txctx, environmentID, configID); err == nil {
			return entity.Conflict("config with snapshots cannot be deleted")
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if _, err := l.configs.Delete(txctx, environmentID, configID); err != nil {
			return err
		}
		_, err := l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.delete", configID))
		return err
	})
}

// lockedResources is the canonical lock order for configuration-domain writes:
// config, structures, then enums. Validation must use only these locked rows.
func (l *ConfigLogic) lockedResources(ctx context.Context, environmentID, configID int64) (*model.Config, []*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	resources := configResources{
		configs:    l.configs,
		structures: l.structures,
		enums:      l.enums,
	}
	return resources.lock(ctx, environmentID, configID)
}

// Detail returns the authorized resource and its derived view.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the authorized resource and derived detail data, or an access, decoding, or schema error.
// Throws: none.
func (l *ConfigLogic) Detail(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*commonv1.Config, *commonv1.TreeNode, error) {
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
	return converted, tree, nil
}

func (l *ConfigLogic) resources(ctx context.Context, environmentID, projectID, configID int64) ([]*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	structures := make([]*commonv1.Structure, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.structures.List(ctx, environmentID, repository.StructureFilter{
			ProjectID: projectID,
			ConfigID:  configID,
		}, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
		if err != nil {
			return nil, nil, err
		}
		converted, err := converter.StructuresToProto(page.Items)
		if err != nil {
			return nil, nil, err
		}
		structures = append(structures, converted...)
		if len(structures) >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}
	enums := make([]*commonv1.ConfigEnum, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.enums.List(ctx, environmentID, repository.ConfigEnumFilter{
			ProjectID: projectID,
			ConfigID:  configID,
		}, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
		if err != nil {
			return nil, nil, err
		}
		converted, err := converter.EnumsToProto(page.Items)
		if err != nil {
			return nil, nil, err
		}
		enums = append(enums, converted...)
		if len(enums) >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}
	return structures, enums, nil
}

func configAudit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: "config",
		ResourceID:   fmt.Sprintf("%d", resourceID),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
