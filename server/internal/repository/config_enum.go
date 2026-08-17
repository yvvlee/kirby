package repository

import (
	"context"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type ConfigEnumFilter struct {
	ProjectID int64
	ConfigID  int64
}

type ConfigEnumUpdate struct {
	Key         string
	Name        string
	Description string
	ValuesJSON  string
	UpdatedBy   int64
	Version     int64
}

type ConfigEnumRepository interface {
	Create(context.Context, int64, int64, *model.ConfigEnum) error
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.ConfigEnum) error
	FindByID(context.Context, int64, int64) (*model.ConfigEnum, error)
	FindByKey(context.Context, int64, int64, string) (*model.ConfigEnum, error)
	List(context.Context, int64, ConfigEnumFilter, base.PageRequest) (base.PageResult[model.ConfigEnum], error)
	Update(context.Context, int64, int64, ConfigEnumUpdate) error
	Delete(context.Context, int64, int64, int64) error
}

type ConfigEnumRepositoryImpl struct {
	engine *xorm.Engine
}

func NewConfigEnumRepository(engine *xorm.Engine) *ConfigEnumRepositoryImpl {
	return &ConfigEnumRepositoryImpl{engine: engine}
}

func (r *ConfigEnumRepositoryImpl) Create(ctx context.Context, environmentID, configID int64, enum *model.ConfigEnum) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	if enum == nil {
		return base.InvalidArgument("config enum is nil")
	}
	if enum.ConfigID != 0 && enum.ConfigID != configID {
		return base.InvalidArgument("config_enum.config_id does not match config_id")
	}
	enum.ConfigID = configID
	result, err := base.Execute(ctx, r.engine, "config enum", insertConfigEnumSQL,
		configEnumCreateArgs(environmentID, configID, enum)...)
	if err != nil {
		return err
	}
	enum.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted config enum id", err)
	}
	return nil
}

func (r *ConfigEnumRepositoryImpl) CreateTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64, enum *model.ConfigEnum) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	if enum == nil {
		return base.InvalidArgument("config enum is nil")
	}
	if enum.ConfigID != 0 && enum.ConfigID != configID {
		return base.InvalidArgument("config_enum.config_id does not match config_id")
	}
	enum.ConfigID = configID
	result, err := base.ExecuteTx(ctx, tx, "config enum", insertConfigEnumSQL,
		configEnumCreateArgs(environmentID, configID, enum)...)
	if err != nil {
		return err
	}
	enum.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted config enum id", err)
	}
	return nil
}

const insertConfigEnumSQL = `
INSERT INTO config_enums
    (config_id, ` + "`key`" + `, name, description, values_json, created_by, updated_by)
SELECT c.id, ?, ?, ?, ?, ?, ?
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE c.id = ? AND c.deleted_at IS NULL AND p.environment_id = ?`

func configEnumCreateArgs(environmentID, configID int64, enum *model.ConfigEnum) []any {
	return []any{
		enum.Key, enum.Name, enum.Description, enum.ValuesJSON,
		enum.CreatedBy, enum.UpdatedBy, configID, environmentID,
	}
}

func (r *ConfigEnumRepositoryImpl) FindByID(ctx context.Context, environmentID, enumID int64) (*model.ConfigEnum, error) {
	if err := validateEnvironmentResource(environmentID, "enum_id", enumID); err != nil {
		return nil, err
	}
	var enum model.ConfigEnum
	err := base.FindOne(ctx, r.engine, "config enum", `
SELECT e.*
FROM config_enums AS e
INNER JOIN configs AS c ON c.id = e.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE p.environment_id = ? AND e.id = ? AND e.deleted_at IS NULL
LIMIT 1`, []any{environmentID, enumID}, &enum)
	if err != nil {
		return nil, err
	}
	return &enum, nil
}

func (r *ConfigEnumRepositoryImpl) FindByKey(ctx context.Context, environmentID, configID int64, key string) (*model.ConfigEnum, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	if key == "" {
		return nil, base.InvalidArgument("config enum key is empty")
	}
	var enum model.ConfigEnum
	err := base.FindOne(ctx, r.engine, "config enum", `
SELECT e.*
FROM config_enums AS e
INNER JOIN configs AS c ON c.id = e.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE p.environment_id = ? AND c.id = ? AND e.`+"`key`"+` = ? AND e.deleted_at IS NULL
LIMIT 1`, []any{environmentID, configID, key}, &enum)
	if err != nil {
		return nil, err
	}
	return &enum, nil
}

func (r *ConfigEnumRepositoryImpl) List(ctx context.Context, environmentID int64, filter ConfigEnumFilter, page base.PageRequest) (base.PageResult[model.ConfigEnum], error) {
	if err := validateEnvironmentResource(environmentID, "project_id", filter.ProjectID); err != nil {
		return base.PageResult[model.ConfigEnum]{}, err
	}
	if err := base.ValidateID("config_id", filter.ConfigID); err != nil {
		return base.PageResult[model.ConfigEnum]{}, err
	}
	page = base.NormalizePage(page)
	args := []any{environmentID, filter.ProjectID, filter.ConfigID}
	from := `
FROM config_enums AS e
INNER JOIN configs AS c ON c.id = e.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE p.environment_id = ? AND p.id = ? AND c.id = ? AND e.deleted_at IS NULL`
	total, err := base.Count(ctx, r.engine, "config enums", "SELECT COUNT(*) AS total"+from, args...)
	if err != nil {
		return base.PageResult[model.ConfigEnum]{}, err
	}
	result := base.PageResult[model.ConfigEnum]{Items: []model.ConfigEnum{}, Total: total, Offset: page.Offset, Limit: page.Limit}
	if total == 0 {
		return result, nil
	}
	listArgs := append(append([]any{}, args...), page.Limit, page.Offset)
	err = base.FindAll(ctx, r.engine, "config enums", "SELECT e.*"+from+`
ORDER BY e.id ASC
LIMIT ? OFFSET ?`, listArgs, &result.Items)
	return result, err
}

func (r *ConfigEnumRepositoryImpl) Update(ctx context.Context, environmentID, enumID int64, update ConfigEnumUpdate) error {
	if err := validateVersionedResource(environmentID, "enum_id", enumID, update.Version); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "config enum", `
UPDATE config_enums AS e
SET e.`+"`key`"+` = ?, e.name = ?, e.description = ?, e.values_json = ?, e.updated_by = ?,
    e.updated_at = UTC_TIMESTAMP(6), e.version = e.version + 1
WHERE e.id = ? AND e.version = ? AND e.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM configs AS c
      INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
      WHERE c.id = e.config_id AND c.deleted_at IS NULL AND p.environment_id = ?
  )`, update.Key, update.Name, update.Description, update.ValuesJSON, update.UpdatedBy,
		enumID, update.Version, environmentID)
	return err
}

func (r *ConfigEnumRepositoryImpl) Delete(ctx context.Context, environmentID, enumID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "enum_id", enumID); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "config enum", `
UPDATE config_enums AS e
SET e.deleted_at = UTC_TIMESTAMP(6), e.updated_by = ?,
    e.updated_at = UTC_TIMESTAMP(6), e.version = e.version + 1
WHERE e.id = ? AND e.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM configs AS c
      INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
      WHERE c.id = e.config_id AND c.deleted_at IS NULL AND p.environment_id = ?
  )`, updatedBy, enumID, environmentID)
	return err
}

var _ ConfigEnumRepository = (*ConfigEnumRepositoryImpl)(nil)
