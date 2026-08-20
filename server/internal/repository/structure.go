package repository

import (
	"context"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type StructureFilter struct {
	ProjectID          int64
	ConfigID           int64
	IgnoreDependencyID *int64
}

type StructureUpdate struct {
	Key         string
	Name        string
	Description string
	FieldsJSON  string
	UpdatedBy   int64
	Version     int64
}

type StructureRepository interface {
	Create(context.Context, int64, int64, *model.Structure) error
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.Structure) error
	FindByID(context.Context, int64, int64) (*model.Structure, error)
	FindByKey(context.Context, int64, int64, string) (*model.Structure, error)
	List(context.Context, int64, StructureFilter, base.PageRequest) (base.PageResult[model.Structure], error)
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.Structure, error)
	Update(context.Context, int64, int64, StructureUpdate) error
	UpdateTx(context.Context, *xorm.Session, int64, int64, StructureUpdate) error
	Delete(context.Context, int64, int64, int64) error
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
	ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.Structure, int64) error
}

func (r *StructureRepositoryImpl) ListForConfigTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64) ([]model.Structure, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, base.InvalidArgument("transaction session is nil")
	}
	locked := make([]model.Structure, 0)
	if err := tx.Context(ctx).SQL(`
SELECT s.*
FROM structures AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND c.id = ?
ORDER BY s.id ASC
FOR UPDATE`, environmentID, configID).Find(&locked); err != nil {
		return nil, base.Wrap("lock structures for config", err)
	}
	items := make([]model.Structure, 0, len(locked))
	for index := range locked {
		if locked[index].DeletedAt.IsZero() {
			items = append(items, locked[index])
		}
	}
	return items, nil
}

type StructureRepositoryImpl struct {
	engine *xorm.Engine
}

func NewStructureRepository(engine *xorm.Engine) *StructureRepositoryImpl {
	return &StructureRepositoryImpl{engine: engine}
}

func (r *StructureRepositoryImpl) Create(ctx context.Context, environmentID, configID int64, structure *model.Structure) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	if structure == nil {
		return base.InvalidArgument("structure is nil")
	}
	if structure.ConfigID != 0 && structure.ConfigID != configID {
		return base.InvalidArgument("structure.config_id does not match config_id")
	}
	structure.ConfigID = configID
	result, err := base.Execute(ctx, r.engine, "structure", insertStructureSQL,
		structureCreateArgs(environmentID, configID, structure)...)
	if err != nil {
		return classifyKeyWriteError("structure", err)
	}
	structure.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted structure id", err)
	}
	return nil
}

func (r *StructureRepositoryImpl) CreateTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64, structure *model.Structure) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	if structure == nil {
		return base.InvalidArgument("structure is nil")
	}
	if structure.ConfigID != 0 && structure.ConfigID != configID {
		return base.InvalidArgument("structure.config_id does not match config_id")
	}
	if tx == nil {
		return base.InvalidArgument("transaction session is nil")
	}
	structure.ConfigID = configID
	if err := lockConfigParent(ctx, tx, environmentID, configID); err != nil {
		return err
	}
	var existing model.Structure
	found, err := tx.Context(ctx).SQL(`
SELECT s.*
FROM structures AS s
WHERE s.config_id = ? AND s.`+"`key`"+` = ?
LIMIT 1
FOR UPDATE`, configID, structure.Key).Get(&existing)
	if err != nil {
		return base.Wrap("lock structure key", err)
	}
	if found {
		if existing.DeletedAt.IsZero() {
			return keyConflict("structure")
		}
		_, err := base.ExecuteTx(ctx, tx, "restore structure", `
UPDATE structures
SET name = ?, description = ?, fields_json = ?, created_by = ?, updated_by = ?,
    created_at = UTC_TIMESTAMP(6), updated_at = UTC_TIMESTAMP(6),
    version = version + 1, deleted_at = NULL
WHERE id = ? AND config_id = ? AND deleted_at IS NOT NULL`,
			structure.Name, structure.Description, structure.FieldsJSON,
			structure.CreatedBy, structure.UpdatedBy, existing.ID, configID)
		if err != nil {
			return classifyKeyWriteError("structure", err)
		}
		structure.ID, structure.Version = existing.ID, existing.Version+1
		return nil
	}
	result, err := base.ExecuteTx(ctx, tx, "structure", insertStructureSQL,
		structureCreateArgs(environmentID, configID, structure)...)
	if err != nil {
		return classifyKeyWriteError("structure", err)
	}
	structure.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted structure id", err)
	}
	return nil
}

const insertStructureSQL = `
INSERT INTO structures
    (config_id, ` + "`key`" + `, name, description, fields_json, created_by, updated_by)
SELECT c.id, ?, ?, ?, ?, ?, ?
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE c.id = ? AND c.deleted_at IS NULL AND e.id = ?`

func structureCreateArgs(environmentID, configID int64, structure *model.Structure) []any {
	return []any{
		structure.Key, structure.Name, structure.Description, structure.FieldsJSON,
		structure.CreatedBy, structure.UpdatedBy, configID, environmentID,
	}
}

func (r *StructureRepositoryImpl) FindByID(ctx context.Context, environmentID, structureID int64) (*model.Structure, error) {
	if err := validateEnvironmentResource(environmentID, "structure_id", structureID); err != nil {
		return nil, err
	}
	var structure model.Structure
	err := base.FindOne(ctx, r.engine, "structure", `
SELECT s.*
FROM structures AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND s.id = ? AND s.deleted_at IS NULL
LIMIT 1`, []any{environmentID, structureID}, &structure)
	if err != nil {
		return nil, err
	}
	return &structure, nil
}

func (r *StructureRepositoryImpl) FindByKey(ctx context.Context, environmentID, configID int64, key string) (*model.Structure, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	if key == "" {
		return nil, base.InvalidArgument("structure key is empty")
	}
	var structure model.Structure
	err := base.FindOne(ctx, r.engine, "structure", `
SELECT s.*
FROM structures AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND c.id = ? AND s.`+"`key`"+` = ? AND s.deleted_at IS NULL
LIMIT 1`, []any{environmentID, configID, key}, &structure)
	if err != nil {
		return nil, err
	}
	return &structure, nil
}

func (r *StructureRepositoryImpl) List(ctx context.Context, environmentID int64, filter StructureFilter, page base.PageRequest) (base.PageResult[model.Structure], error) {
	if err := validateEnvironmentResource(environmentID, "project_id", filter.ProjectID); err != nil {
		return base.PageResult[model.Structure]{}, err
	}
	if err := base.ValidateID("config_id", filter.ConfigID); err != nil {
		return base.PageResult[model.Structure]{}, err
	}
	where := "e.id = ? AND p.id = ? AND c.id = ? AND s.deleted_at IS NULL"
	args := []any{environmentID, filter.ProjectID, filter.ConfigID}
	if filter.IgnoreDependencyID != nil {
		if err := base.ValidateID("ignore_dependency_id", *filter.IgnoreDependencyID); err != nil {
			return base.PageResult[model.Structure]{}, err
		}
		where += " AND s.id <> ?"
		args = append(args, *filter.IgnoreDependencyID)
	}
	return r.list(ctx, where, args, page)
}

func (r *StructureRepositoryImpl) list(ctx context.Context, where string, args []any, page base.PageRequest) (base.PageResult[model.Structure], error) {
	page = base.NormalizePage(page)
	from := `
FROM structures AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE ` + where
	total, err := base.Count(ctx, r.engine, "structures", "SELECT COUNT(*) AS total"+from, args...)
	if err != nil {
		return base.PageResult[model.Structure]{}, err
	}
	result := base.PageResult[model.Structure]{Items: []model.Structure{}, Total: total, Offset: page.Offset, Limit: page.Limit}
	if total == 0 {
		return result, nil
	}
	listArgs := append(append([]any{}, args...), page.Limit, page.Offset)
	err = base.FindAll(ctx, r.engine, "structures", "SELECT s.*"+from+`
ORDER BY s.id ASC
LIMIT ? OFFSET ?`, listArgs, &result.Items)
	return result, err
}

func (r *StructureRepositoryImpl) Update(ctx context.Context, environmentID, structureID int64, update StructureUpdate) error {
	if err := validateVersionedResource(environmentID, "structure_id", structureID, update.Version); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "structure", `
UPDATE structures AS s
SET s.`+"`key`"+` = ?, s.name = ?, s.description = ?, s.fields_json = ?, s.updated_by = ?,
    s.updated_at = UTC_TIMESTAMP(6), s.version = s.version + 1
WHERE s.id = ? AND s.version = ? AND s.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM configs AS c
      INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
      INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
      WHERE c.id = s.config_id AND c.deleted_at IS NULL AND e.id = ?
  )`, update.Key, update.Name, update.Description, update.FieldsJSON, update.UpdatedBy,
		structureID, update.Version, environmentID)
	return err
}

func (r *StructureRepositoryImpl) UpdateTx(ctx context.Context, tx *xorm.Session, environmentID, structureID int64, update StructureUpdate) error {
	if err := validateVersionedResource(environmentID, "structure_id", structureID, update.Version); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "structure", `
UPDATE structures AS s
SET s.`+"`key`"+` = ?, s.name = ?, s.description = ?, s.fields_json = ?, s.updated_by = ?,
    s.updated_at = UTC_TIMESTAMP(6), s.version = s.version + 1
WHERE s.id = ? AND s.version = ? AND s.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM configs AS c
      INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
      INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
      WHERE c.id = s.config_id AND c.deleted_at IS NULL AND e.id = ?
  )`, update.Key, update.Name, update.Description, update.FieldsJSON, update.UpdatedBy,
		structureID, update.Version, environmentID)
	return err
}

func (r *StructureRepositoryImpl) Delete(ctx context.Context, environmentID, structureID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "structure_id", structureID); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "structure", `
UPDATE structures AS s
SET s.deleted_at = UTC_TIMESTAMP(6), s.updated_by = ?,
    s.updated_at = UTC_TIMESTAMP(6), s.version = s.version + 1
WHERE s.id = ? AND s.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM configs AS c
      INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
      INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
      WHERE c.id = s.config_id AND c.deleted_at IS NULL AND e.id = ?
  )`, updatedBy, structureID, environmentID)
	return err
}

func (r *StructureRepositoryImpl) DeleteTx(ctx context.Context, tx *xorm.Session, environmentID, structureID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "structure_id", structureID); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "structure", `
UPDATE structures AS s
SET s.deleted_at = UTC_TIMESTAMP(6), s.updated_by = ?,
    s.updated_at = UTC_TIMESTAMP(6), s.version = s.version + 1
WHERE s.id = ? AND s.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM configs AS c
      INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
      INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
      WHERE c.id = s.config_id AND c.deleted_at IS NULL AND e.id = ?
  )`, updatedBy, structureID, environmentID)
	return err
}

func (r *StructureRepositoryImpl) ReconcileTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64, structures []*model.Structure, actorID int64) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	if actorID <= 0 {
		return base.InvalidArgument("actor_id must be greater than zero")
	}
	var locked struct {
		ID int64 `xorm:"id"`
	}
	if err := base.LockOne(ctx, tx, "config", `
SELECT c.id
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND c.id = ? AND c.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{environmentID, configID}, &locked); err != nil {
		return err
	}
	existing := make([]model.Structure, 0)
	if err := tx.Context(ctx).SQL(`
SELECT s.*
FROM structures AS s
WHERE s.config_id = ?
ORDER BY s.id ASC
FOR UPDATE`, configID).Find(&existing); err != nil {
		return base.Wrap("lock structures for reconciliation", err)
	}
	byKey := make(map[string]*model.Structure, len(existing))
	for index := range existing {
		byKey[existing[index].Key] = &existing[index]
	}
	desired := make(map[string]struct{}, len(structures))
	for _, structure := range structures {
		if structure == nil || structure.Key == "" {
			return base.InvalidArgument("structure and key are required")
		}
		if _, duplicate := desired[structure.Key]; duplicate {
			return base.InvalidArgument("structure keys must be unique")
		}
		desired[structure.Key] = struct{}{}
		structure.ConfigID = configID
		structure.UpdatedBy = actorID
		current := byKey[structure.Key]
		if current == nil {
			structure.ID = 0
			structure.CreatedBy = actorID
			if err := r.CreateTx(ctx, tx, environmentID, configID, structure); err != nil {
				return err
			}
			continue
		}
		if _, err := base.ExecuteTx(ctx, tx, "structure", `
UPDATE structures
SET name = ?, description = ?, fields_json = ?, deleted_at = NULL, updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND config_id = ?`, structure.Name, structure.Description, structure.FieldsJSON,
			actorID, current.ID, configID); err != nil {
			return err
		}
		structure.ID = current.ID
		structure.Version = current.Version + 1
	}
	for index := range existing {
		current := &existing[index]
		if _, keep := desired[current.Key]; keep || !current.DeletedAt.IsZero() {
			continue
		}
		if _, err := base.ExecuteTx(ctx, tx, "structure", `
UPDATE structures
SET deleted_at = UTC_TIMESTAMP(6), updated_by = ?, updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND config_id = ? AND deleted_at IS NULL`, actorID, current.ID, configID); err != nil {
			return err
		}
	}
	return nil
}

var _ StructureRepository = (*StructureRepositoryImpl)(nil)
