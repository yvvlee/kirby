package repository

import (
	"context"
	"strings"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type ConfigFilter struct {
	ProjectID  *int64
	ProjectKey *string
	Key        *string
}

type ConfigUpdate struct {
	Description string
	IsArray     bool
	TypeJSON    string
	UpdatedBy   int64
	Version     int64
}

type ConfigValueUpdate struct {
	Value     string
	UpdatedBy int64
	Version   int64
}

type ConfigRepository interface {
	Create(context.Context, int64, int64, *model.Config) error
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.Config) error
	FindByID(context.Context, int64, int64) (*model.Config, error)
	FindByKey(context.Context, int64, int64, string) (*model.Config, error)
	List(context.Context, int64, ConfigFilter, base.PageRequest) (base.PageResult[model.Config], error)
	Update(context.Context, int64, int64, ConfigUpdate) error
	UpdateTx(context.Context, *xorm.Session, int64, int64, ConfigUpdate) error
	UpdateValue(context.Context, int64, int64, ConfigValueUpdate) error
	UpdateValueTx(context.Context, *xorm.Session, int64, int64, ConfigValueUpdate) error
	Delete(context.Context, int64, int64, int64) error
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Config, error)
}

type ConfigRepositoryImpl struct {
	engine *xorm.Engine
}

func NewConfigRepository(engine *xorm.Engine) *ConfigRepositoryImpl {
	return &ConfigRepositoryImpl{engine: engine}
}

func (r *ConfigRepositoryImpl) Create(ctx context.Context, environmentID, projectID int64, config *model.Config) error {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return err
	}
	if config == nil {
		return base.InvalidArgument("config is nil")
	}
	if config.ProjectID != 0 && config.ProjectID != projectID {
		return base.InvalidArgument("config.project_id does not match project_id")
	}
	config.ProjectID = projectID
	result, err := base.Execute(ctx, r.engine, "config", insertConfigSQL,
		configCreateArgs(environmentID, projectID, config)...)
	if err != nil {
		return classifyKeyWriteError("config", err)
	}
	config.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted config id", err)
	}
	return nil
}

func (r *ConfigRepositoryImpl) CreateTx(ctx context.Context, tx *xorm.Session, environmentID, projectID int64, config *model.Config) error {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return err
	}
	if config == nil {
		return base.InvalidArgument("config is nil")
	}
	if config.ProjectID != 0 && config.ProjectID != projectID {
		return base.InvalidArgument("config.project_id does not match project_id")
	}
	if tx == nil {
		return base.InvalidArgument("transaction session is nil")
	}
	config.ProjectID = projectID
	var project struct {
		ID int64 `xorm:"id"`
	}
	if err := base.LockOne(ctx, tx, "project", `
SELECT p.id
FROM projects AS p
WHERE p.environment_id = ? AND p.id = ? AND p.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{environmentID, projectID}, &project); err != nil {
		return err
	}
	var existing model.Config
	found, err := tx.Context(ctx).SQL(`
SELECT c.*
FROM configs AS c
WHERE c.project_id = ? AND c.`+"`key`"+` = ?
LIMIT 1
FOR UPDATE`, projectID, config.Key).Get(&existing)
	if err != nil {
		return base.Wrap("lock config key", err)
	}
	if found {
		if existing.DeletedAt.IsZero() {
			return keyConflict("config")
		}
		lockedStructures := make([]struct {
			ID int64 `xorm:"id"`
		}, 0)
		if err := tx.Context(ctx).SQL(`
SELECT s.id
FROM structures AS s
WHERE s.config_id = ?
ORDER BY s.id ASC
FOR UPDATE`, existing.ID).Find(&lockedStructures); err != nil {
			return base.Wrap("lock structures before restoring config", err)
		}
		lockedEnums := make([]struct {
			ID int64 `xorm:"id"`
		}, 0)
		if err := tx.Context(ctx).SQL(`
SELECT e.id
FROM config_enums AS e
WHERE e.config_id = ?
ORDER BY e.id ASC
FOR UPDATE`, existing.ID).Find(&lockedEnums); err != nil {
			return base.Wrap("lock config enums before restoring config", err)
		}
		if _, err := tx.Context(ctx).Exec(`
UPDATE structures
SET deleted_at = UTC_TIMESTAMP(6), updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE config_id = ? AND deleted_at IS NULL`, config.UpdatedBy, existing.ID); err != nil {
			return base.Wrap("reset structures before restoring config", err)
		}
		if _, err := tx.Context(ctx).Exec(`
UPDATE config_enums
SET deleted_at = UTC_TIMESTAMP(6), updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE config_id = ? AND deleted_at IS NULL`, config.UpdatedBy, existing.ID); err != nil {
			return base.Wrap("reset config enums before restoring config", err)
		}
		_, err := base.ExecuteTx(ctx, tx, "restore config", `
UPDATE configs
SET description = ?, is_array = ?, type_json = ?, value = ?, runtime_version = ?,
    created_by = ?, updated_by = ?, created_at = UTC_TIMESTAMP(6),
    updated_at = UTC_TIMESTAMP(6), version = version + 1, deleted_at = NULL
WHERE id = ? AND project_id = ? AND deleted_at IS NOT NULL`,
			config.Description, config.IsArray, config.TypeJSON, config.Value, config.RuntimeVersion,
			config.CreatedBy, config.UpdatedBy, existing.ID, projectID)
		if err != nil {
			return classifyKeyWriteError("config", err)
		}
		config.ID, config.Version = existing.ID, existing.Version+1
		return nil
	}
	result, err := base.ExecuteTx(ctx, tx, "config", insertConfigSQL,
		configCreateArgs(environmentID, projectID, config)...)
	if err != nil {
		return classifyKeyWriteError("config", err)
	}
	config.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted config id", err)
	}
	return nil
}

const insertConfigSQL = `
INSERT INTO configs
    (project_id, ` + "`key`" + `, description, is_array, type_json, value, runtime_version, created_by, updated_by)
SELECT p.id, ?, ?, ?, ?, ?, ?, ?, ?
FROM projects AS p
WHERE p.id = ? AND p.environment_id = ? AND p.deleted_at IS NULL`

func configCreateArgs(environmentID, projectID int64, config *model.Config) []any {
	return []any{
		config.Key, config.Description, config.IsArray, config.TypeJSON, config.Value,
		config.RuntimeVersion, config.CreatedBy, config.UpdatedBy, projectID, environmentID,
	}
}

func (r *ConfigRepositoryImpl) FindByID(ctx context.Context, environmentID, configID int64) (*model.Config, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	var config model.Config
	err := base.FindOne(ctx, r.engine, "config", configByIDSQL, []any{environmentID, configID}, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ConfigRepositoryImpl) FindByKey(ctx context.Context, environmentID, projectID int64, key string) (*model.Config, error) {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return nil, base.InvalidArgument("config key is empty")
	}
	var config model.Config
	err := base.FindOne(ctx, r.engine, "config", `
SELECT c.*
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE p.environment_id = ? AND p.id = ? AND c.`+"`key`"+` = ? AND c.deleted_at IS NULL
LIMIT 1`, []any{environmentID, projectID, key}, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ConfigRepositoryImpl) List(ctx context.Context, environmentID int64, filter ConfigFilter, page base.PageRequest) (base.PageResult[model.Config], error) {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return base.PageResult[model.Config]{}, err
	}
	where, args, err := configListFilter(environmentID, filter)
	if err != nil {
		return base.PageResult[model.Config]{}, err
	}
	page = base.NormalizePage(page)
	total, err := base.Count(ctx, r.engine, "configs", `
SELECT COUNT(*) AS total
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE `+where, args...)
	if err != nil {
		return base.PageResult[model.Config]{}, err
	}
	result := base.PageResult[model.Config]{Items: []model.Config{}, Total: total, Offset: page.Offset, Limit: page.Limit}
	if total == 0 {
		return result, nil
	}
	listArgs := append(append([]any{}, args...), page.Limit, page.Offset)
	err = base.FindAll(ctx, r.engine, "configs", `
SELECT c.*
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE `+where+`
ORDER BY c.id DESC
LIMIT ? OFFSET ?`, listArgs, &result.Items)
	return result, err
}

func (r *ConfigRepositoryImpl) Update(ctx context.Context, environmentID, configID int64, update ConfigUpdate) error {
	if err := validateVersionedResource(environmentID, "config_id", configID, update.Version); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "config", `
UPDATE configs AS c
SET c.description = ?, c.is_array = ?, c.type_json = ?, c.updated_by = ?,
    c.updated_at = UTC_TIMESTAMP(6), c.version = c.version + 1
WHERE c.id = ? AND c.version = ? AND c.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      WHERE p.id = c.project_id AND p.environment_id = ? AND p.deleted_at IS NULL
  )`, update.Description, update.IsArray, update.TypeJSON, update.UpdatedBy,
		configID, update.Version, environmentID)
	return err
}

func (r *ConfigRepositoryImpl) UpdateTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64, update ConfigUpdate) error {
	if err := validateVersionedResource(environmentID, "config_id", configID, update.Version); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "config", `
UPDATE configs AS c
SET c.description = ?, c.is_array = ?, c.type_json = ?, c.updated_by = ?,
    c.updated_at = UTC_TIMESTAMP(6), c.version = c.version + 1
WHERE c.id = ? AND c.version = ? AND c.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      WHERE p.id = c.project_id AND p.environment_id = ? AND p.deleted_at IS NULL
  )`, update.Description, update.IsArray, update.TypeJSON, update.UpdatedBy,
		configID, update.Version, environmentID)
	return err
}

func (r *ConfigRepositoryImpl) UpdateValue(ctx context.Context, environmentID, configID int64, update ConfigValueUpdate) error {
	if err := validateVersionedResource(environmentID, "config_id", configID, update.Version); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "config", `
UPDATE configs AS c
SET c.value = ?, c.updated_by = ?, c.updated_at = UTC_TIMESTAMP(6), c.version = c.version + 1
WHERE c.id = ? AND c.version = ? AND c.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      WHERE p.id = c.project_id AND p.environment_id = ? AND p.deleted_at IS NULL
  )`, update.Value, update.UpdatedBy, configID, update.Version, environmentID)
	return err
}

func (r *ConfigRepositoryImpl) UpdateValueTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64, update ConfigValueUpdate) error {
	if err := validateVersionedResource(environmentID, "config_id", configID, update.Version); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "config", `
UPDATE configs AS c
SET c.value = ?, c.updated_by = ?, c.updated_at = UTC_TIMESTAMP(6), c.version = c.version + 1
WHERE c.id = ? AND c.version = ? AND c.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      WHERE p.id = c.project_id AND p.environment_id = ? AND p.deleted_at IS NULL
  )`, update.Value, update.UpdatedBy, configID, update.Version, environmentID)
	return err
}

func (r *ConfigRepositoryImpl) Delete(ctx context.Context, environmentID, configID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "config", `
UPDATE configs AS c
SET c.deleted_at = UTC_TIMESTAMP(6), c.updated_by = ?,
    c.updated_at = UTC_TIMESTAMP(6), c.version = c.version + 1
WHERE c.id = ? AND c.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      WHERE p.id = c.project_id AND p.environment_id = ? AND p.deleted_at IS NULL
  )`, updatedBy, configID, environmentID)
	return err
}

func (r *ConfigRepositoryImpl) DeleteTx(ctx context.Context, tx *xorm.Session, environmentID, configID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "config", `
UPDATE configs AS c
SET c.deleted_at = UTC_TIMESTAMP(6), c.updated_by = ?,
    c.updated_at = UTC_TIMESTAMP(6), c.version = c.version + 1
WHERE c.id = ? AND c.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      WHERE p.id = c.project_id AND p.environment_id = ? AND p.deleted_at IS NULL
  )`, updatedBy, configID, environmentID)
	return err
}

func (r *ConfigRepositoryImpl) LockByID(ctx context.Context, tx *xorm.Session, environmentID, configID int64) (*model.Config, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	var config model.Config
	err := base.LockOne(ctx, tx, "config", configByIDSQL+"\nFOR UPDATE", []any{environmentID, configID}, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

const configByIDSQL = `
SELECT c.*
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE p.environment_id = ? AND c.id = ? AND c.deleted_at IS NULL
LIMIT 1`

func configListFilter(environmentID int64, filter ConfigFilter) (string, []any, error) {
	where := "p.environment_id = ? AND c.deleted_at IS NULL"
	args := []any{environmentID}
	if filter.ProjectID != nil {
		if err := base.ValidateID("project_id", *filter.ProjectID); err != nil {
			return "", nil, err
		}
		where += " AND p.id = ?"
		args = append(args, *filter.ProjectID)
	}
	if filter.ProjectKey != nil {
		if strings.TrimSpace(*filter.ProjectKey) == "" {
			return "", nil, base.InvalidArgument("project key is empty")
		}
		where += " AND p.`key` = ?"
		args = append(args, *filter.ProjectKey)
	}
	if filter.Key != nil {
		where += " AND c.`key` = ?"
		args = append(args, *filter.Key)
	}
	return where, args, nil
}

func validateVersionedResource(environmentID int64, resourceName string, resourceID, version int64) error {
	if err := validateEnvironmentResource(environmentID, resourceName, resourceID); err != nil {
		return err
	}
	if version < 0 {
		return base.InvalidArgument("version must not be negative")
	}
	return nil
}

var _ ConfigRepository = (*ConfigRepositoryImpl)(nil)
