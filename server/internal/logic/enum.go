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

type EnumLogic struct {
	enums        repository.ConfigEnumRepository
	configs      repository.ConfigRepository
	structures   repository.StructureRepository
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
}

// NewEnumLogic creates EnumLogic with its configuration dependencies.
//
// Parameters: enums locks and reconciles enum definitions; configs reads and writes configuration rows; structures locks and reconciles structure definitions; permissions checks the caller permissions; audits records successful changes in the transaction; transactions commits state changes and audit events together.
// Returns: *EnumLogic.
// Throws: none.
func NewEnumLogic(
	enums repository.ConfigEnumRepository,
	configs repository.ConfigRepository,
	structures repository.StructureRepository,
	permissions environmentPermissionChecker,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
) *EnumLogic {
	return &EnumLogic{
		enums:        enums,
		configs:      configs,
		structures:   structures,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
	}
}

// Create creates the resource within the authorized scope.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, and the parent configuration, enum key, labels, and options.
// Returns: the created resource; returns permission, schema-validation, or transactional write failures.
// Throws: none.
func (l *EnumLogic) Create(ctx context.Context, param *entity.CreateEnumParam) (*commonv1.ConfigEnum, error) {
	actor, environmentID := param.Actor, param.EnvironmentID
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnumWrite); err != nil {
		return nil, err
	}
	valuesJSON, err := converter.EncodeOptions(param.Values)
	if err != nil {
		return nil, entity.Invalid("invalid enum values")
	}
	item := &model.ConfigEnum{
		ConfigID:    param.ConfigID,
		Key:         param.Key,
		Name:        param.Name,
		Description: param.Description,
		ValuesJSON:  valuesJSON,
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		_, structures, enums, err := l.lockedResources(txctx, environmentID, param.ConfigID)
		if err != nil {
			return err
		}
		enums = append(enums, &commonv1.ConfigEnum{
			Key:         param.Key,
			Name:        param.Name,
			Description: param.Description,
			Values:      param.Values,
		})
		if _, err := entity.NewSchema(structures, enums); err != nil {
			return err
		}
		if _, err := l.enums.Create(txctx, environmentID, param.ConfigID, item); err != nil {
			return err
		}
		_, err = l.audits.RecordForEnvironment(txctx, environmentID, enumAudit(actor, "enum.create", item.ID))
		return err
	})
	if err != nil {
		return nil, err
	}
	created, err := l.enums.FindByID(ctx, environmentID, item.ID)
	if err != nil {
		return nil, err
	}
	return converter.EnumToProto(created)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, the enum ID, replacement key and options, and expected version.
// Returns: the reloaded resource after commit; returns permission, schema, dependency-conflict, or stale-version errors.
// Throws: none.
func (l *EnumLogic) Update(ctx context.Context, param *entity.UpdateEnumParam) (*commonv1.ConfigEnum, error) {
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
	updated, err := l.enums.FindByID(ctx, param.EnvironmentID, param.EnumID)
	if err != nil {
		return nil, err
	}
	return converter.EnumToProto(updated)
}

type enumUpdateOperation struct {
	configID    int64
	input       *entity.UpdateEnumParam
	contentJSON string
}

func (l *EnumLogic) prepareUpdate(ctx context.Context, actor permission.Actor, input *entity.UpdateEnumParam) (enumUpdateOperation, error) {
	if err := l.permissions.Require(ctx, actor.UserID, input.EnvironmentID, permission.EnumWrite); err != nil {
		return enumUpdateOperation{}, err
	}
	current, err := l.enums.FindByID(ctx, input.EnvironmentID, input.EnumID)
	if err != nil {
		return enumUpdateOperation{}, err
	}
	contentJSON, err := converter.EncodeOptions(input.Values)
	if err != nil {
		return enumUpdateOperation{}, entity.Invalid("invalid enum values")
	}
	return enumUpdateOperation{
		configID:    current.ConfigID,
		input:       input,
		contentJSON: contentJSON,
	}, nil
}

func (l *EnumLogic) validateUpdate(ctx context.Context, environmentID int64, operation enumUpdateOperation) error {
	config, structures, enums, err := l.lockedResources(ctx, environmentID, operation.configID)
	if err != nil {
		return err
	}
	locked, err := enumForUpdate(config, structures, enums, operation.input)
	if err != nil {
		return err
	}
	return validateEnumReplacement(structures, enums, locked, operation.input)
}

func enumForUpdate(config *model.Config, structures []*commonv1.Structure, enums []*commonv1.ConfigEnum, input *entity.UpdateEnumParam) (*commonv1.ConfigEnum, error) {
	locked, err := findEnum(enums, input.EnumID)
	if err != nil {
		return nil, err
	}
	referenced, err := referencedEnum(config, structures, locked.Key)
	if err != nil {
		return nil, err
	}
	if locked.Key != input.Key && referenced {
		return nil, entity.Conflict("referenced enum key cannot be changed")
	}
	return locked, nil
}

func validateEnumReplacement(structures []*commonv1.Structure, enums []*commonv1.ConfigEnum, locked *commonv1.ConfigEnum, input *entity.UpdateEnumParam) error {
	candidate, err := enumUpdateCandidate(locked, input)
	if err != nil {
		return err
	}
	for index, item := range enums {
		if item.Id == locked.Id {
			enums[index] = candidate
		}
	}
	// Draft values may lag behind a schema edit; snapshots and publishing validate values.
	_, err = entity.NewSchema(structures, enums)
	return err
}

func enumUpdateCandidate(locked *commonv1.ConfigEnum, input *entity.UpdateEnumParam) (*commonv1.ConfigEnum, error) {
	return &commonv1.ConfigEnum{
		Id:          locked.Id,
		ConfigId:    locked.ConfigId,
		Key:         input.Key,
		Name:        input.Name,
		Description: input.Description,
		Values:      input.Values,
		Version:     locked.Version,
	}, nil
}

func (l *EnumLogic) saveUpdate(ctx context.Context, actor permission.Actor, environmentID int64, operation enumUpdateOperation) error {
	input := operation.input
	if _, err := l.enums.Update(ctx, environmentID, input.EnumID, repository.ConfigEnumUpdate{
		Key:         input.Key,
		Name:        input.Name,
		Description: input.Description,
		ValuesJSON:  operation.contentJSON,
		UpdatedBy:   actor.UserID,
		Version:     input.Version,
	}); err != nil {
		return err
	}
	_, err := l.audits.RecordForEnvironment(ctx, environmentID, enumAudit(actor, "enum.update", input.EnumID))
	return err
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; projectID selects the project; configID selects the configuration.
// Returns: the authorized records, or a permission, repository, or conversion error.
// Throws: none.
func (l *EnumLogic) List(ctx context.Context, actor permission.Actor, environmentID, projectID, configID int64) ([]*commonv1.ConfigEnum, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnumRead); err != nil {
		return nil, err
	}
	config, err := l.configs.FindByID(ctx, environmentID, configID)
	if err != nil {
		return nil, err
	}
	if config.ProjectID != projectID {
		return nil, repository.Missing("config")
	}
	items := make([]model.ConfigEnum, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.enums.List(ctx, environmentID, repository.ConfigEnumFilter{
			ProjectID: projectID,
			ConfigID:  configID,
		}, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if len(items) >= int(page.Total) || len(page.Items) == 0 {
			return converter.EnumsToProto(items)
		}
	}
}

// Delete removes the resource after checking ownership and dependent state.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; enumID identifies the enum to delete.
// Returns: nil after committing the deletion and audit event; dependency conflicts and permission or write failures are returned.
// Throws: none.
func (l *EnumLogic) Delete(ctx context.Context, actor permission.Actor, environmentID, enumID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnumWrite); err != nil {
		return err
	}
	current, err := l.enums.FindByID(ctx, environmentID, enumID)
	if err != nil {
		return err
	}
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		config, structures, enums, err := l.lockedResources(txctx, environmentID, current.ConfigID)
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
		if _, err := l.enums.Delete(txctx, environmentID, enumID); err != nil {
			return err
		}
		_, err = l.audits.RecordForEnvironment(txctx, environmentID, enumAudit(actor, "enum.delete", enumID))
		return err
	})
}

func (l *EnumLogic) lockedResources(ctx context.Context, environmentID, configID int64) (*model.Config, []*commonv1.Structure, []*commonv1.ConfigEnum, error) {
	resources := configResources{
		configs:    l.configs,
		structures: l.structures,
		enums:      l.enums,
	}
	return resources.lock(ctx, environmentID, configID)
}

func findEnum(items []*commonv1.ConfigEnum, id int64) (*commonv1.ConfigEnum, error) {
	for _, item := range items {
		if item.Id == id {
			return item, nil
		}
	}
	return nil, repository.Missing("config enum")
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
		if fieldsReferenceEnum(structure.Fields, key) {
			return true, nil
		}
	}
	return false, nil
}

func enumAudit(actor permission.Actor, action string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: "config_enum",
		ResourceID:   fmt.Sprintf("%d", resourceID),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
