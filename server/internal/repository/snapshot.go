package repository

import (
	"context"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type SnapshotFilter struct {
	ProjectID int64
	ConfigID  int64
	Status    *model.SnapshotStatus
	IsUsing   *bool
}

type SnapshotRepository interface {
	Create(ctx context.Context, environmentID, projectID, configID int64, snapshot *model.Snapshot) error
	FindByID(ctx context.Context, environmentID, snapshotID int64) (*model.Snapshot, error)
	List(ctx context.Context, environmentID int64, filter SnapshotFilter, page PageRequest) (PageResult[model.Snapshot], error)
	FindReleasedForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error)
	FindCurrentForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error)
	FindAnyForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error)
	ListReleasedConfigIDs(ctx context.Context, environmentID, projectID int64) ([]int64, error)
	Delete(ctx context.Context, environmentID, snapshotID, updatedBy int64) error
	LockByID(ctx context.Context, environmentID, snapshotID int64) (*model.Snapshot, error)
	SetCurrent(ctx context.Context, environmentID, configID, snapshotID, updatedBy int64) error
}

type SnapshotRepositoryImpl struct {
	*lorm.Repository[*model.Snapshot]
}

// NewSnapshotRepository creates SnapshotRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *SnapshotRepositoryImpl.
// Throws: none.
func NewSnapshotRepository(storage *infra.Storage) *SnapshotRepositoryImpl {
	return &SnapshotRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Snapshot](),
	}
}

// Create 创建快照，初始状态为未发布且未启用。
//
// Parameters: ctx carries cancellation and transaction state; projectID selects the project; configID selects the configuration; snapshot contains the snapshot content and metadata.
// Returns: nil after inserting an unreleased, unselected snapshot with its ID, version, and timestamps; insertion failures are returned.
// Throws: none.
func (r *SnapshotRepositoryImpl) Create(ctx context.Context, _, projectID, configID int64, snapshot *model.Snapshot) error {
	snapshot.ProjectID = projectID
	snapshot.ConfigID = configID
	snapshot.Status = model.SnapshotStatusUnreleased
	snapshot.IsUsing = false
	snapshot.PrepareInsert()
	_, err := r.Insert(ctx, snapshot)
	return err
}

// FindByID loads a resource only when it belongs to the requested scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; snapshotID selects the snapshot.
// Returns: the live row, or ErrNotFound when the ID is missing, deleted, or outside the environment.
// Throws: none.
func (r *SnapshotRepositoryImpl) FindByID(ctx context.Context, environmentID, snapshotID int64) (*model.Snapshot, error) {
	snapshot, has, err := snapshotScope(r.Engine).
		Where(builder.Eq{
			"e.id": environmentID,
			"s.id": snapshotID,
		}).
		Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, Missing("snapshot")
	}
	return snapshot, nil
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; filter restricts the returned rows; page contains pagination bounds.
// Returns: the scoped page, total row count, normalized offset and limit, or a database error.
// Throws: none.
func (r *SnapshotRepositoryImpl) List(ctx context.Context, environmentID int64, filter SnapshotFilter, page PageRequest) (PageResult[model.Snapshot], error) {
	cond := builder.And{
		builder.Eq{
			"e.id": environmentID,
			"p.id": filter.ProjectID,
			"c.id": filter.ConfigID,
		},
		eqIf("s.status", filter.Status),
		eqIf("s.is_using", filter.IsUsing),
	}
	return paginate[model.Snapshot](ctx, snapshotScope(r.Engine).Where(cond).OrderBy("s.id DESC"), page)
}

// FindReleasedForConfig finds the published snapshot in the requested configuration scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the newest released snapshot, or ErrNotFound if none is visible in the scope.
// Throws: none.
func (r *SnapshotRepositoryImpl) FindReleasedForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfig(ctx, environmentID, configID, builder.Eq{"s.status": model.SnapshotStatusReleased}, "released snapshot")
}

// FindCurrentForConfig finds the current snapshot in the requested configuration scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the current snapshot, or ErrNotFound if no selection is visible in the scope.
// Throws: none.
func (r *SnapshotRepositoryImpl) FindCurrentForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfig(ctx, environmentID, configID, builder.Eq{"s.is_using": true}, "current snapshot")
}

// FindAnyForConfig checks for snapshots before allowing configuration deletion.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: the newest live snapshot, or ErrNotFound when deletion has no snapshot dependency.
// Throws: none.
func (r *SnapshotRepositoryImpl) FindAnyForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfig(ctx, environmentID, configID, nil, "snapshot")
}

func (r *SnapshotRepositoryImpl) findForConfig(ctx context.Context, environmentID, configID int64, cond builder.Sqlizer, resource string) (*model.Snapshot, error) {
	db := snapshotScope(r.Engine).Where(builder.Eq{
		"e.id": environmentID,
		"c.id": configID,
	})
	if cond != nil {
		db = db.Where(cond)
	}
	snapshot, has, err := db.OrderBy("s.id DESC").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, Missing(resource)
	}
	return snapshot, nil
}

// ListReleasedConfigIDs lists configuration IDs that have a visible published snapshot.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; projectID selects the project.
// Returns: distinct configuration IDs with live released snapshots, in ascending order.
// Throws: none.
func (r *SnapshotRepositoryImpl) ListReleasedConfigIDs(ctx context.Context, environmentID, projectID int64) ([]int64, error) {
	ids, err := snapshotScope(r.Engine).Select("s.config_id").Distinct().Where(builder.Eq{
		"e.id":     environmentID,
		"p.id":     projectID,
		"s.status": model.SnapshotStatusReleased,
	}).Asc("s.config_id").FindCols[int64](ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return ids, nil
}

// Delete removes the resource after checking ownership and dependent state.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; snapshotID selects the snapshot; updatedBy identifies the actor recorded on the write.
// Returns: nil after soft deletion; ErrNotFound for an inaccessible row or ErrNoRowsAffected for a rejected write.
// Throws: none.
func (r *SnapshotRepositoryImpl) Delete(ctx context.Context, environmentID, snapshotID, updatedBy int64) error {
	fields := new(model.Snapshot).LormCols()
	if _, err := r.FindByID(ctx, environmentID, snapshotID); err != nil {
		return err
	}
	affected, err := r.Engine.Update[*model.Snapshot]().Where(builder.IsNull(fields.DeletedAt())).ID(snapshotID).
		Where(builder.Eq{fields.Status(): model.SnapshotStatusUnreleased}).
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
		return Unchanged("snapshot")
	}
	return nil
}

// LockByID locks a scoped resource so concurrent writes share the same order.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; snapshotID selects the snapshot.
// Returns: the locked live row, or ErrNotFound for a missing or inaccessible resource; requires the caller’s transaction.
// Throws: none.
func (r *SnapshotRepositoryImpl) LockByID(ctx context.Context, environmentID, snapshotID int64) (*model.Snapshot, error) {
	snapshot, has, err := snapshotScope(r.Engine).
		Where(builder.Eq{
			"e.id": environmentID,
			"s.id": snapshotID,
		}).
		Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, Missing("snapshot")
	}
	return snapshot, nil
}

// SetCurrent 将目标快照标记为使用中，同配置下其余快照取消使用。
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; snapshotID selects the snapshot; updatedBy identifies the actor recorded on the write.
// Returns: nil after clearing the previous selection and selecting the target; returns ownership or write failures.
// Throws: none.
func (r *SnapshotRepositoryImpl) SetCurrent(ctx context.Context, environmentID, configID, snapshotID, updatedBy int64) error {
	fields := new(model.Snapshot).LormCols()
	config, err := r.findScopedConfig(ctx, environmentID, configID)
	if err != nil {
		return err
	}
	_, has, err := r.Engine.Query[*model.Snapshot]().Where(builder.IsNull(fields.DeletedAt())).
		Where(builder.Eq{
			fields.ID():        snapshotID,
			fields.ConfigID():  configID,
			fields.ProjectID(): config.ProjectID,
		}).
		Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !has {
		return Missing("snapshot")
	}
	now := time.Now().UTC()
	if _, err = r.Engine.Update[*model.Snapshot]().Where(builder.IsNull(fields.DeletedAt())).
		Where(builder.Eq{
			fields.ConfigID():  configID,
			fields.ProjectID(): config.ProjectID,
		}).
		Where(builder.NotEq{fields.ID(): snapshotID}).
		Where(builder.Eq{fields.IsUsing(): true}).
		Set(fields.Version(), builder.Expr("version + 1")).
		SetMap(map[string]any{
			fields.IsUsing():   false,
			fields.UpdatedBy(): updatedBy,
			fields.UpdatedAt(): now,
		}).Exec(ctx); err != nil {
		return WrapDatabaseError(err)
	}
	affected, err := r.Engine.Update[*model.Snapshot]().Where(builder.IsNull(fields.DeletedAt())).ID(snapshotID).
		Where(builder.Eq{
			fields.ConfigID():  configID,
			fields.ProjectID(): config.ProjectID,
		}).
		Set(fields.Version(), builder.Expr("version + 1")).
		SetMap(map[string]any{
			fields.IsUsing():   true,
			fields.UpdatedBy(): updatedBy,
			fields.UpdatedAt(): now,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("current snapshot")
	}
	return nil
}

func (r *SnapshotRepositoryImpl) findScopedConfig(ctx context.Context, environmentID, configID int64) (*model.Config, error) {
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

var _ SnapshotRepository = (*SnapshotRepositoryImpl)(nil)
