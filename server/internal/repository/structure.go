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
	Update(context.Context, int64, int64, StructureUpdate) error
	Delete(context.Context, int64, int64, int64) error
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
		return err
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
	structure.ConfigID = configID
	result, err := base.ExecuteTx(ctx, tx, "structure", insertStructureSQL,
		structureCreateArgs(environmentID, configID, structure)...)
	if err != nil {
		return err
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
WHERE c.id = ? AND c.deleted_at IS NULL AND p.environment_id = ?`

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
WHERE p.environment_id = ? AND s.id = ? AND s.deleted_at IS NULL
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
WHERE p.environment_id = ? AND c.id = ? AND s.`+"`key`"+` = ? AND s.deleted_at IS NULL
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
	where := "p.environment_id = ? AND p.id = ? AND c.id = ? AND s.deleted_at IS NULL"
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
      WHERE c.id = s.config_id AND c.deleted_at IS NULL AND p.environment_id = ?
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
      WHERE c.id = s.config_id AND c.deleted_at IS NULL AND p.environment_id = ?
  )`, updatedBy, structureID, environmentID)
	return err
}

var _ StructureRepository = (*StructureRepositoryImpl)(nil)
