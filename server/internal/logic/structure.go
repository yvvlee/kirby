package logic

import (
	"context"
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type StructureLogic struct {
	structures   repository.StructureRepository
	configs      repository.ConfigRepository
	enums        repository.ConfigEnumRepository
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
}

// NewStructureLogic creates StructureLogic with its configuration dependencies.
//
// Parameters: structures locks and reconciles structure definitions; configs reads and writes configuration rows; enums locks and reconciles enum definitions; permissions checks the caller permissions; audits records successful changes in the transaction; transactions commits state changes and audit events together.
// Returns: *StructureLogic.
// Throws: none.
func NewStructureLogic(
	structures repository.StructureRepository,
	configs repository.ConfigRepository,
	enums repository.ConfigEnumRepository,
	permissions environmentPermissionChecker,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
) *StructureLogic {
	return &StructureLogic{
		structures:   structures,
		configs:      configs,
		enums:        enums,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
	}
}

// Create creates the resource within the authorized scope.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; input contains the parent configuration, structure key, labels, and fields.
// Returns: the created resource; returns permission, schema-validation, or transactional write failures.
// Throws: none.
func (l *StructureLogic) Create(ctx context.Context, param *entity.CreateStructureParam) (*commonv1.Structure, error) {
	actor, environmentID := param.Actor, param.EnvironmentID
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.StructureWrite); err != nil {
		return nil, err
	}
	item := &model.Structure{
		ConfigID:    param.ConfigID,
		Key:         param.Key,
		Name:        param.Name,
		Description: param.Description,
		FieldsJSON:  "[]",
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		_, structures, enums, err := l.lockedResources(txctx, environmentID, param.ConfigID)
		if err != nil {
			return err
		}
		structures = append(structures, &commonv1.Structure{
			Key:         param.Key,
			Name:        param.Name,
			Description: param.Description,
		})
		if _, err := entity.NewSchema(structures, enums); err != nil {
			return err
		}
		if _, err := l.structures.Create(txctx, environmentID, param.ConfigID, item); err != nil {
			return err
		}
		_, err = l.audits.RecordForEnvironment(txctx, environmentID, structureAudit(actor, "structure.create", item.ID))
		return err
	})
	if err != nil {
		return nil, err
	}
	created, err := l.structures.FindByID(ctx, environmentID, item.ID)
	if err != nil {
		return nil, err
	}
	return converter.StructureToProto(created)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, the structure ID, replacement key and fields, and expected version.
// Returns: the reloaded resource after commit; returns permission, schema, dependency-conflict, or stale-version errors.
// Throws: none.
func (l *StructureLogic) Update(ctx context.Context, param *entity.UpdateStructureParam) (*commonv1.Structure, error) {
	actor := param.Actor
	operation, err := l.prepareUpdate(ctx, actor, param)
	if err != nil {
		return nil, err
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		if err := l.validateUpdate(txctx, param.EnvironmentID, operation); err != nil {
			return err
		}
		return l.saveUpdate(txctx, actor, param.EnvironmentID, operation)
	})
	if err != nil {
		return nil, err
	}
	updated, err := l.structures.FindByID(ctx, param.EnvironmentID, param.StructureID)
	if err != nil {
		return nil, err
	}
	return converter.StructureToProto(updated)
}

type structureUpdateOperation struct {
	configID    int64
	input       *entity.UpdateStructureParam
	contentJSON string
}

func (l *StructureLogic) prepareUpdate(ctx context.Context, actor permission.Actor, input *entity.UpdateStructureParam) (structureUpdateOperation, error) {
	if err := l.permissions.Require(ctx, actor.UserID, input.EnvironmentID, permission.StructureWrite); err != nil {
		return structureUpdateOperation{}, err
	}
	current, err := l.structures.FindByID(ctx, input.EnvironmentID, input.StructureID)
	if err != nil {
		return structureUpdateOperation{}, err
	}
	contentJSON, err := converter.EncodeFields(input.Fields)
	if err != nil {
		return structureUpdateOperation{}, entity.Invalid("invalid structure fields")
	}
	return structureUpdateOperation{
		configID:    current.ConfigID,
		input:       input,
		contentJSON: contentJSON,
	}, nil
}

func (l *StructureLogic) validateUpdate(ctx context.Context, environmentID int64, operation structureUpdateOperation) error {
	config, structures, enums, err := l.lockedResources(ctx, environmentID, operation.configID)
	if err != nil {
		return err
	}
	locked, err := structureForUpdate(config, structures, operation.input)
	if err != nil {
		return err
	}
	return validateStructureReplacement(structures, enums, locked, operation.input)
}

func structureForUpdate(config *model.Config, structures []*commonv1.Structure, input *entity.UpdateStructureParam) (*commonv1.Structure, error) {
	locked, err := findStructure(structures, input.StructureID)
	if err != nil {
		return nil, err
	}
	referenced, err := referencedStructure(config, structures, locked.Key, locked.Id)
	if err != nil {
		return nil, err
	}
	if locked.Key != input.Key && referenced {
		return nil, entity.Conflict("referenced structure key cannot be changed")
	}
	return locked, nil
}

func validateStructureReplacement(structures []*commonv1.Structure, enums []*commonv1.ConfigEnum, locked *commonv1.Structure, input *entity.UpdateStructureParam) error {
	candidate, err := structureUpdateCandidate(locked, input)
	if err != nil {
		return err
	}
	for index, item := range structures {
		if item.Id == locked.Id {
			structures[index] = candidate
		}
	}
	// Draft values may lag behind a schema edit; snapshots and publishing validate values.
	_, err = entity.NewSchema(structures, enums)
	return err
}

func structureUpdateCandidate(locked *commonv1.Structure, input *entity.UpdateStructureParam) (*commonv1.Structure, error) {
	return &commonv1.Structure{
		Id:          locked.Id,
		ConfigId:    locked.ConfigId,
		Key:         input.Key,
		Name:        input.Name,
		Description: input.Description,
		Fields:      input.Fields,
		Version:     uint32(input.Version),
	}, nil
}

func (l *StructureLogic) saveUpdate(ctx context.Context, actor permission.Actor, environmentID int64, operation structureUpdateOperation) error {
	input := operation.input
	if _, err := l.structures.Update(ctx, environmentID, input.StructureID, repository.StructureUpdate{
		Key:         input.Key,
		Name:        input.Name,
		Description: input.Description,
		FieldsJSON:  operation.contentJSON,
		UpdatedBy:   actor.UserID,
		Version:     input.Version,
	}); err != nil {
		return err
	}
	_, err := l.audits.RecordForEnvironment(ctx, environmentID, structureAudit(actor, "structure.update", input.StructureID))
	return err
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, and the project and configuration selection that optionally excludes one structure.
// Returns: the authorized records, or a permission or repository error.
// Throws: none.
func (l *StructureLogic) List(ctx context.Context, param *entity.ListStructureParam) ([]*commonv1.Structure, error) {
	if err := l.permissions.Require(ctx, param.Actor.UserID, param.EnvironmentID, permission.StructureRead); err != nil {
		return nil, err
	}
	config, err := l.configs.FindByID(ctx, param.EnvironmentID, param.ConfigID)
	if err != nil {
		return nil, err
	}
	if config.ProjectID != param.ProjectID {
		return nil, repository.Missing("config")
	}
	items, err := l.allStructures(ctx, param.EnvironmentID, param.ProjectID, param.ConfigID)
	if err != nil {
		return nil, err
	}
	if param.IgnoreID != nil {
		return entity.FilterStructureChoices(items, *param.IgnoreID)
	}
	return items, nil
}

// Delete removes the resource after checking ownership and dependent state.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; structureID identifies the structure to delete.
// Returns: nil after committing the deletion and audit event; dependency conflicts and permission or write failures are returned.
// Throws: none.
func (l *StructureLogic) Delete(ctx context.Context, actor permission.Actor, environmentID, structureID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.StructureWrite); err != nil {
		return err
	}
	current, err := l.structures.FindByID(ctx, environmentID, structureID)
	if err != nil {
		return err
	}
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		config, structures, enums, err := l.lockedResources(txctx, environmentID, current.ConfigID)
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
		if _, err := l.structures.Delete(txctx, environmentID, structureID); err != nil {
			return err
		}
		_, err = l.audits.RecordForEnvironment(txctx, environmentID, structureAudit(actor, "structure.delete", structureID))
		return err
	})
}

// lockedResources uses the same order as config and enum writes.
func (l *StructureLogic) lockedResources(ctx context.Context, environmentID, configID int64) (*model.Config, []*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	resources := configResources{
		configs:    l.configs,
		structures: l.structures,
		enums:      l.enums,
	}
	return resources.lock(ctx, environmentID, configID)
}

func findStructure(items []*commonv1.Structure, id int64) (*commonv1.Structure, error) {
	for _, item := range items {
		if item.Id == id {
			return item, nil
		}
	}
	return nil, repository.Missing("structure")
}

func (l *StructureLogic) allStructures(ctx context.Context, environmentID, projectID, configID int64) ([]*commonv1.Structure, error) {
	items := make([]*commonv1.Structure, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.structures.List(ctx, environmentID, repository.StructureFilter{
			ProjectID: projectID,
			ConfigID:  configID,
		}, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
		if err != nil {
			return nil, err
		}
		converted, err := converter.StructuresToProto(page.Items)
		if err != nil {
			return nil, err
		}
		items = append(items, converted...)
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
		if fieldsReferenceStructure(item.Fields, key) {
			return true, nil
		}
	}
	return false, nil
}

func structureAudit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: "structure",
		ResourceID:   fmt.Sprintf("%d", resourceID),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
