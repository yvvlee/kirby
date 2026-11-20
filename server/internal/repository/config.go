package repository

import (
	"context"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
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
	Create(ctx context.Context, environmentID, projectID int64, config *model.Config) (rowsAffected int64, err error)
	FindByID(ctx context.Context, environmentID, configID int64) (*model.Config, error)
	List(ctx context.Context, environmentID int64, filter ConfigFilter, page PageRequest) (PageResult[model.Config], error)
	Update(ctx context.Context, environmentID, configID int64, update ConfigUpdate) (rowsAffected int64, err error)
	UpdateValue(ctx context.Context, environmentID, configID int64, update ConfigValueUpdate) (rowsAffected int64, err error)
	Delete(ctx context.Context, environmentID, configID int64) (rowsAffected int64, err error)
	LockByID(ctx context.Context, environmentID, configID int64) (*model.Config, error)
}

type ConfigRepositoryImpl struct {
	*lorm.Repository[*model.Config]
}

// NewConfigRepository creates ConfigRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *ConfigRepositoryImpl.
// Throws: none.
func NewConfigRepository(storage *infra.Storage) *ConfigRepositoryImpl {
	return &ConfigRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Config](),
	}
}

// Create 创建配置，同名 key 冲突由唯一键约束拒绝。
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; projectID selects the project; config contains the new configuration key, schema, and value.
// Returns: rowsAffected from the insert; IDs, version, and timestamps are copied into the supplied record. Key conflicts and database failures are returned.
// Throws: none.
func (r *ConfigRepositoryImpl) Create(ctx context.Context, environmentID, projectID int64, config *model.Config) (rowsAffected int64, err error) {
	if err = lockScopedProject(ctx, r.Engine, environmentID, projectID); err != nil {
		return
	}
	config.EnvironmentID = environmentID
	config.ProjectID = projectID
	rowsAffected, err = r.Insert(ctx, config)
	err = classifyKeyWriteError("config", err)
	return
}

// FindByID loads a resource only when it belongs to the requested scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the live row, or ErrNotFound when the ID is missing, deleted, or outside the environment.
// Throws: none.
func (r *ConfigRepositoryImpl) FindByID(ctx context.Context, environmentID, configID int64) (*model.Config, error) {
	config, has, err := configScope(r.Engine).
		Where(builder.Eq{
			"e.id": environmentID,
			"c.id": configID,
		}).
		Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, Missing("config")
	}
	return config, nil
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; filter restricts the returned rows; page contains pagination bounds.
// Returns: the scoped page, total row count, normalized offset and limit, or a database error.
// Throws: none.
func (r *ConfigRepositoryImpl) List(ctx context.Context, environmentID int64, filter ConfigFilter, page PageRequest) (PageResult[model.Config], error) {
	cond := builder.And{
		builder.Eq{"e.id": environmentID},
		eqIf("p.id", filter.ProjectID),
		eqIf("p.key", filter.ProjectKey),
		eqIf("c.key", filter.Key),
	}
	return paginate[model.Config](ctx, configScope(r.Engine).Where(cond).OrderBy("c.id DESC"), page)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; update contains the guarded changes and expected version.
// Returns: rowsAffected after one matching live row changes; ErrNoRowsAffected if the expected version is stale.
// Throws: none.
func (r *ConfigRepositoryImpl) Update(ctx context.Context, environmentID, configID int64, update ConfigUpdate) (rowsAffected int64, err error) {
	fields := new(model.Config).LormCols()
	if _, err = r.FindByID(ctx, environmentID, configID); err != nil {
		return
	}
	rowsAffected, err = r.Engine.Update[*model.Config]().ID(configID).
		Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Description(): update.Description,
			fields.IsArray():     update.IsArray,
			fields.TypeJSON():    update.TypeJSON,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("config")
		return
	}
	return
}

// UpdateValue validates and updates the draft configuration value.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; update contains the guarded changes and expected version.
// Returns: rowsAffected after the guarded value write; ErrNoRowsAffected if the row was deleted or its version changed.
// Throws: none.
func (r *ConfigRepositoryImpl) UpdateValue(ctx context.Context, environmentID, configID int64, update ConfigValueUpdate) (rowsAffected int64, err error) {
	fields := new(model.Config).LormCols()
	if _, err = r.FindByID(ctx, environmentID, configID); err != nil {
		return
	}
	rowsAffected, err = r.Engine.Update[*model.Config]().ID(configID).
		Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Value():     update.Value,
			fields.UpdatedBy(): update.UpdatedBy,
			fields.UpdatedAt(): time.Now(),
			fields.Version():   update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("config")
		return
	}
	return
}

// Delete 物理删除配置并级联删除其下全部结构与枚举。
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: rowsAffected from deleting the configuration row; ErrNotFound for an inaccessible row.
// Throws: none.
func (r *ConfigRepositoryImpl) Delete(ctx context.Context, environmentID, configID int64) (rowsAffected int64, err error) {
	if _, err = r.FindByID(ctx, environmentID, configID); err != nil {
		return
	}
	if err = deleteConfigChildren(ctx, r.Engine, configID); err != nil {
		return
	}
	rowsAffected, err = r.Engine.Delete[*model.Config]().ID(configID).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("config")
		return
	}
	return
}

// deleteConfigChildren 物理删除配置下所有结构与枚举。
func deleteConfigChildren(ctx context.Context, engine *lorm.Engine, configID int64) error {
	_, err := engine.Delete[*model.Structure]().Where(builder.Eq{"config_id": configID}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	_, err = engine.Delete[*model.ConfigEnum]().Where(builder.Eq{"config_id": configID}).Exec(ctx)
	return WrapDatabaseError(err)
}

// LockByID locks a scoped resource so concurrent writes share the same order.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the locked live row, or ErrNotFound for a missing or inaccessible resource; requires the caller’s transaction.
// Throws: none.
func (r *ConfigRepositoryImpl) LockByID(ctx context.Context, environmentID, configID int64) (*model.Config, error) {
	config, has, err := configScope(r.Engine).
		Where(builder.Eq{
			"e.id": environmentID,
			"c.id": configID,
		}).
		Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, Missing("config")
	}
	return config, nil
}

var _ ConfigRepository = (*ConfigRepositoryImpl)(nil)
