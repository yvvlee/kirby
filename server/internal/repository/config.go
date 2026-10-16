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
	Create(ctx context.Context, environmentID, projectID int64, config *model.Config) error
	FindByID(ctx context.Context, environmentID, configID int64) (*model.Config, error)
	List(ctx context.Context, environmentID int64, filter ConfigFilter, page PageRequest) (PageResult[model.Config], error)
	Update(ctx context.Context, environmentID, configID int64, update ConfigUpdate) error
	UpdateValue(ctx context.Context, environmentID, configID int64, update ConfigValueUpdate) error
	Delete(ctx context.Context, environmentID, configID, updatedBy int64) error
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

// Create 创建配置。软删除的同key记录会被恢复，其下结构与枚举被一并软删除重置。
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; projectID selects the project; config contains the new configuration key, schema, and value.
// Returns: nil after insertion or restoration; IDs, version, and timestamps are copied into the supplied record. Key conflicts and database failures are returned.
// Throws: none.
func (r *ConfigRepositoryImpl) Create(ctx context.Context, environmentID, projectID int64, config *model.Config) error {
	if err := lockScopedProject(ctx, r.Engine, environmentID, projectID); err != nil {
		return err
	}
	existing, err := lockConfigByKey(ctx, r.Engine, projectID, config.Key)
	if err != nil {
		return err
	}
	if existing != nil {
		return r.restore(ctx, existing, config)
	}
	config.PrepareInsert()
	_, err = r.Insert(ctx, config)
	return classifyKeyWriteError("config", err)
}

func (r *ConfigRepositoryImpl) restore(ctx context.Context, existing, config *model.Config) error {
	fields := new(model.Config).LormCols()
	if existing.DeletedAt == nil {
		return keyConflict("config")
	}
	if err := resetConfigChildren(ctx, r.Engine, existing.ID, config.UpdatedBy); err != nil {
		return err
	}
	now := time.Now().UTC()
	affected, err := r.Engine.Update[*model.Config]().ID(existing.ID).SetMap(map[string]any{
		fields.Description():    config.Description,
		fields.IsArray():        config.IsArray,
		fields.TypeJSON():       config.TypeJSON,
		fields.Value():          config.Value,
		fields.RuntimeVersion(): config.RuntimeVersion,
		fields.CreatedBy():      config.CreatedBy,
		fields.UpdatedBy():      config.UpdatedBy,
		fields.CreatedAt():      now,
		fields.UpdatedAt():      now,
		fields.Version():        existing.Version + 1,
		fields.DeletedAt():      nil,
	}).Exec(ctx)
	if err != nil {
		return classifyKeyWriteError("config", err)
	}
	if affected == 0 {
		return Unchanged("config")
	}
	config.ID = existing.ID
	config.Version = existing.Version + 1
	config.CreatedAt = now
	config.UpdatedAt = now
	return nil
}

// lockConfigByKey 锁定(project_id, key)对应的配置行，包含软删除记录。
func lockConfigByKey(ctx context.Context, engine *lorm.Engine, projectID int64, key string) (*model.Config, error) {
	fields := new(model.Config).LormCols()
	existing, has, err := engine.Query[*model.Config]().
		Where(builder.Eq{
			fields.ProjectID(): projectID,
			fields.Key():       key,
		}).
		Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, nil
	}
	return existing, nil
}

// resetConfigChildren 软删除配置下所有存活的结构与枚举。
func resetConfigChildren(ctx context.Context, engine *lorm.Engine, configID, updatedBy int64) error {
	now := time.Now().UTC()
	values := map[string]any{
		"deleted_at": now,
		"updated_by": updatedBy,
		"updated_at": now,
		"version":    builder.Expr("version + 1"),
	}
	_, err := engine.Update[*model.Structure]().Where(builder.IsNull("deleted_at")).
		Where(builder.Eq{"config_id": configID}).SetMap(values).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	_, err = engine.Update[*model.ConfigEnum]().Where(builder.IsNull("deleted_at")).
		Where(builder.Eq{"config_id": configID}).SetMap(values).Exec(ctx)
	return WrapDatabaseError(err)
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
// Returns: nil after one matching live row changes; ErrNoRowsAffected if the expected version is stale.
// Throws: none.
func (r *ConfigRepositoryImpl) Update(ctx context.Context, environmentID, configID int64, update ConfigUpdate) error {
	fields := new(model.Config).LormCols()
	if _, err := r.FindByID(ctx, environmentID, configID); err != nil {
		return err
	}
	affected, err := r.Engine.Update[*model.Config]().Where(builder.IsNull(fields.DeletedAt())).ID(configID).
		Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Description(): update.Description,
			fields.IsArray():     update.IsArray,
			fields.TypeJSON():    update.TypeJSON,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now().UTC(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("config")
	}
	return nil
}

// UpdateValue validates and updates the draft configuration value.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; update contains the guarded changes and expected version.
// Returns: nil after the guarded value write; ErrNoRowsAffected if the row was deleted or its version changed.
// Throws: none.
func (r *ConfigRepositoryImpl) UpdateValue(ctx context.Context, environmentID, configID int64, update ConfigValueUpdate) error {
	fields := new(model.Config).LormCols()
	if _, err := r.FindByID(ctx, environmentID, configID); err != nil {
		return err
	}
	affected, err := r.Engine.Update[*model.Config]().Where(builder.IsNull(fields.DeletedAt())).ID(configID).
		Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Value():     update.Value,
			fields.UpdatedBy(): update.UpdatedBy,
			fields.UpdatedAt(): time.Now().UTC(),
			fields.Version():   update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("config")
	}
	return nil
}

// Delete removes the resource after checking ownership and dependent state.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; updatedBy identifies the actor recorded on the write.
// Returns: nil after soft deletion; ErrNotFound for an inaccessible row or ErrNoRowsAffected for a rejected write.
// Throws: none.
func (r *ConfigRepositoryImpl) Delete(ctx context.Context, environmentID, configID, updatedBy int64) error {
	fields := new(model.Config).LormCols()
	if _, err := r.FindByID(ctx, environmentID, configID); err != nil {
		return err
	}
	affected, err := r.Engine.Update[*model.Config]().Where(builder.IsNull(fields.DeletedAt())).ID(configID).
		Set(fields.Version(), builder.Expr("version + 1")).
		SetMap(map[string]any{
			fields.DeletedAt(): time.Now().UTC(),
			fields.UpdatedBy(): updatedBy,
			fields.UpdatedAt(): time.Now().UTC(),
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("config")
	}
	return nil
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
