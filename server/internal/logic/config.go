package logic

import (
	"context"
	"errors"
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type ConfigCreateInput struct {
	ProjectID   int64
	Key         string
	Description string
}

type ConfigUpdateInput struct {
	ConfigID    int64
	Description string
	FieldType   *commonv1.Field_Type
	IsArray     bool
	Version     int64
}

type ConfigValueInput struct {
	ConfigID int64
	Value    string
	Version  int64
}

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
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; input contains the project ID, new configuration key, and description.
// Returns: the created resource; returns permission, schema-validation, or transactional write failures.
// Throws: none.
func (l *ConfigLogic) Create(ctx context.Context, actor permission.Actor, environmentID int64, input ConfigCreateInput) (*model.Config, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite); err != nil {
		return nil, err
	}
	fieldType := &commonv1.Field_Type{Kind: &commonv1.Field_Type_BaseType{BaseType: commonv1.Field_STRING}}
	typeJSON, err := converter.EncodeFieldType(fieldType)
	if err != nil {
		return nil, err
	}
	item := &model.Config{
		ProjectID:   input.ProjectID,
		Key:         input.Key,
		Description: input.Description,
		TypeJSON:    typeJSON,
		Value:       `""`,
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		if err := l.configs.Create(txctx, environmentID, input.ProjectID, item); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.create", item.ID))
	})
	if err != nil {
		return nil, err
	}
	// A write-only role may create a config because the returned value is the
	// known empty initial value. Do not re-read after commit and accidentally
	// expose a value changed concurrently by a reader/editor.
	return item, nil
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; input contains the configuration ID, replacement schema metadata, and expected version.
// Returns: the reloaded resource after commit; returns permission, schema, dependency-conflict, or stale-version errors.
// Throws: none.
func (l *ConfigLogic) Update(ctx context.Context, actor permission.Actor, environmentID int64, input ConfigUpdateInput) (*model.Config, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite, permission.ConfigRead); err != nil {
		return nil, err
	}
	typeJSON, err := converter.EncodeFieldType(input.FieldType)
	if err != nil {
		return nil, entity.Invalid("invalid config type")
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		_, structures, enums, err := l.lockedResources(txctx, environmentID, input.ConfigID)
		if err != nil {
			return err
		}
		schema, err := entity.NewSchema(structures, enums)
		if err != nil {
			return err
		}
		// Type changes may intentionally make the current draft value invalid.
		// UpdateValue and snapshot creation are the explicit validation gates.
		if _, err := schema.DefaultValue(input.FieldType, input.IsArray); err != nil {
			return err
		}
		if err := l.configs.Update(txctx, environmentID, input.ConfigID, repository.ConfigUpdate{
			Description: input.Description,
			IsArray:     input.IsArray,
			TypeJSON:    typeJSON,
			UpdatedBy:   actor.UserID,
			Version:     input.Version,
		}); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.update", input.ConfigID))
	})
	if err != nil {
		return nil, err
	}
	return l.configs.FindByID(ctx, environmentID, input.ConfigID)
}

// UpdateValue validates and updates the draft configuration value.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; input contains the configuration ID, replacement JSON value, and expected version.
// Returns: the reloaded configuration after validation and commit, or a permission, value-validation, or stale-version error.
// Throws: none.
func (l *ConfigLogic) UpdateValue(ctx context.Context, actor permission.Actor, environmentID int64, input ConfigValueInput) (*model.Config, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigWrite, permission.ConfigRead); err != nil {
		return nil, err
	}
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		current, structures, enums, err := l.lockedResources(txctx, environmentID, input.ConfigID)
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
		candidate.Value = input.Value
		if err := schema.ValidateConfig(candidate); err != nil {
			return err
		}
		if err := l.configs.UpdateValue(txctx, environmentID, input.ConfigID, repository.ConfigValueUpdate{
			Value:     input.Value,
			UpdatedBy: actor.UserID,
			Version:   input.Version,
		}); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.value.update", input.ConfigID))
	})
	if err != nil {
		return nil, err
	}
	return l.configs.FindByID(ctx, environmentID, input.ConfigID)
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; filter restricts the returned rows.
// Returns: the authorized records, or a permission or repository error.
// Throws: none.
func (l *ConfigLogic) List(ctx context.Context, actor permission.Actor, environmentID int64, filter repository.ConfigFilter) ([]model.Config, map[int64]bool, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ConfigRead); err != nil {
		return nil, nil, err
	}
	items := make([]model.Config, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.configs.List(ctx, environmentID, filter, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
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
		if err := l.configs.Delete(txctx, environmentID, configID, actor.UserID); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, configAudit(actor, "config.delete", configID))
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
func (l *ConfigLogic) Detail(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Config, *commonv1.TreeNode, error) {
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
