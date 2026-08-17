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
	ListForConfigTx(context.Context, *xorm.Session, int64, int64) ([]model.ConfigEnum, error)
	Update(context.Context, int64, int64, ConfigEnumUpdate) error
	UpdateTx(context.Context, *xorm.Session, int64, int64, ConfigEnumUpdate) error
	Delete(context.Context, int64, int64, int64) error
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
	ReconcileTx(context.Context, *xorm.Session, int64, int64, []*model.ConfigEnum, int64) error
}

func (r *ConfigEnumRepositoryImpl) ListForConfigTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64) ([]model.ConfigEnum, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, base.InvalidArgument("transaction session is nil")
	}
	items := make([]model.ConfigEnum, 0)
	if err := tx.Context(ctx).SQL(`
SELECT e.*
FROM config_enums AS e
INNER JOIN configs AS c ON c.id = e.config_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
WHERE p.environment_id = ? AND c.id = ? AND e.deleted_at IS NULL
ORDER BY e.id ASC
FOR UPDATE`, environmentID, configID).Find(&items); err != nil {
		return nil, base.Wrap("lock config enums for config", err)
	}
	return items, nil
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

func (r *ConfigEnumRepositoryImpl) UpdateTx(ctx context.Context, tx *xorm.Session, environmentID, enumID int64, update ConfigEnumUpdate) error {
	if err := validateVersionedResource(environmentID, "enum_id", enumID, update.Version); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "config enum", `
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

func (r *ConfigEnumRepositoryImpl) DeleteTx(ctx context.Context, tx *xorm.Session, environmentID, enumID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "enum_id", enumID); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "config enum", `
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

func (r *ConfigEnumRepositoryImpl) ReconcileTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64, enums []*model.ConfigEnum, actorID int64) error {
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
WHERE p.environment_id = ? AND c.id = ? AND c.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{environmentID, configID}, &locked); err != nil {
		return err
	}
	existing := make([]model.ConfigEnum, 0)
	if err := tx.Context(ctx).SQL(`
SELECT e.*
FROM config_enums AS e
WHERE e.config_id = ?
ORDER BY e.id ASC
FOR UPDATE`, configID).Find(&existing); err != nil {
		return base.Wrap("lock config enums for reconciliation", err)
	}
	byKey := make(map[string]*model.ConfigEnum, len(existing))
	for index := range existing {
		byKey[existing[index].Key] = &existing[index]
	}
	desired := make(map[string]struct{}, len(enums))
	for _, item := range enums {
		if item == nil || item.Key == "" {
			return base.InvalidArgument("config enum and key are required")
		}
		if _, duplicate := desired[item.Key]; duplicate {
			return base.InvalidArgument("config enum keys must be unique")
		}
		desired[item.Key] = struct{}{}
		item.ConfigID = configID
		item.UpdatedBy = actorID
		current := byKey[item.Key]
		if current == nil {
			item.ID = 0
			item.CreatedBy = actorID
			if err := r.CreateTx(ctx, tx, environmentID, configID, item); err != nil {
				return err
			}
			continue
		}
		if _, err := base.ExecuteTx(ctx, tx, "config enum", `
UPDATE config_enums
SET name = ?, description = ?, values_json = ?, deleted_at = NULL, updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND config_id = ?`, item.Name, item.Description, item.ValuesJSON,
			actorID, current.ID, configID); err != nil {
			return err
		}
		item.ID = current.ID
		item.Version = current.Version + 1
	}
	for index := range existing {
		current := &existing[index]
		if _, keep := desired[current.Key]; keep || !current.DeletedAt.IsZero() {
			continue
		}
		if _, err := base.ExecuteTx(ctx, tx, "config enum", `
UPDATE config_enums
SET deleted_at = UTC_TIMESTAMP(6), updated_by = ?, updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND config_id = ? AND deleted_at IS NULL`, actorID, current.ID, configID); err != nil {
			return err
		}
	}
	return nil
}

var _ ConfigEnumRepository = (*ConfigEnumRepositoryImpl)(nil)
