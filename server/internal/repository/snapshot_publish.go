package repository

import (
	"context"
	"math"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

// SnapshotPublicationRepository 包含直接发布流程专用的受保护写操作。
type SnapshotPublicationUpdate struct {
	EnvironmentID   int64
	ConfigID        int64
	SnapshotID      int64
	ExpectedVersion int64
	UpdatedBy       int64
	ChangedAt       time.Time
}

type SnapshotPublicationRepository interface {
	LockForConfig(ctx context.Context, environmentID, configID int64) ([]model.Snapshot, error)
	SetReleased(ctx context.Context, update SnapshotPublicationUpdate) error
	SetUnreleased(ctx context.Context, update SnapshotPublicationUpdate) error
	IncrementRuntimeVersion(ctx context.Context, environmentID, configID int64) error
}

type SnapshotPublicationRepositoryImpl struct {
	*lorm.Repository[*model.Snapshot]
}

// NewSnapshotPublicationRepository creates SnapshotPublicationRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *SnapshotPublicationRepositoryImpl.
// Throws: none.
func NewSnapshotPublicationRepository(storage *infra.Storage) *SnapshotPublicationRepositoryImpl {
	return &SnapshotPublicationRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Snapshot](),
	}
}

// LockForConfig 按ID升序锁定配置下全部存活快照。
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: live snapshots locked in ascending ID order, or a database error.
// Throws: none.
func (r *SnapshotPublicationRepositoryImpl) LockForConfig(ctx context.Context, environmentID, configID int64) ([]model.Snapshot, error) {
	items, err := snapshotScope(r.Engine).
		Where(builder.Eq{
			"e.id": environmentID,
			"c.id": configID,
		}).
		OrderBy("s.id ASC").Suffix("FOR UPDATE").
		Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(items), nil
}

// SetReleased publishes a snapshot only if its status and version still match.
//
// Parameters: ctx carries cancellation and transaction state; update contains the guarded changes and expected version.
// Returns: nil after the unreleased status and expected version match; otherwise ErrNoRowsAffected.
// Throws: none.
func (r *SnapshotPublicationRepositoryImpl) SetReleased(ctx context.Context, update SnapshotPublicationUpdate) error {
	update.ChangedAt = update.ChangedAt.UTC()
	fields := new(model.Snapshot).LormCols()
	affected, err := r.Engine.Update[*model.Snapshot]().Where(builder.IsNull(fields.DeletedAt())).ID(update.SnapshotID).
		Where(builder.Eq{
			fields.ConfigID(): update.ConfigID,
			fields.Status():   model.SnapshotStatusUnreleased,
			fields.Version():  update.ExpectedVersion,
		}).
		SetMap(map[string]any{
			fields.Status():      model.SnapshotStatusReleased,
			fields.PublishedAt(): update.ChangedAt,
			fields.PublishedBy(): update.UpdatedBy,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   update.ChangedAt,
			fields.Version():     update.ExpectedVersion + 1,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("snapshot publication")
	}
	return nil
}

// SetUnreleased unpublishes a snapshot only if its status and version still match.
//
// Parameters: ctx carries cancellation and transaction state; update contains the guarded changes and expected version.
// Returns: nil after the released status and expected version match; otherwise ErrNoRowsAffected.
// Throws: none.
func (r *SnapshotPublicationRepositoryImpl) SetUnreleased(ctx context.Context, update SnapshotPublicationUpdate) error {
	update.ChangedAt = update.ChangedAt.UTC()
	fields := new(model.Snapshot).LormCols()
	affected, err := r.Engine.Update[*model.Snapshot]().Where(builder.IsNull(fields.DeletedAt())).ID(update.SnapshotID).
		Where(builder.Eq{
			fields.ConfigID(): update.ConfigID,
			fields.Status():   model.SnapshotStatusReleased,
			fields.Version():  update.ExpectedVersion,
		}).
		SetMap(map[string]any{
			fields.Status():      model.SnapshotStatusUnreleased,
			fields.PublishedAt(): nil,
			fields.PublishedBy(): nil,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   update.ChangedAt,
			fields.Version():     update.ExpectedVersion + 1,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("snapshot unpublication")
	}
	return nil
}

// IncrementRuntimeVersion advances the runtime cache generation without integer overflow.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: nil after advancing the runtime generation and UTC update time while retaining the edit version; ErrNoRowsAffected at the integer limit.
// Throws: none.
func (r *SnapshotPublicationRepositoryImpl) IncrementRuntimeVersion(ctx context.Context, environmentID, configID int64) error {
	fields := new(model.Config).LormCols()
	if err := lockScopedConfig(ctx, r.Engine, environmentID, configID); err != nil {
		return err
	}
	affected, err := r.Engine.Update[*model.Config]().Where(builder.IsNull(fields.DeletedAt())).ID(configID).
		Where(builder.Lt("runtime_version", int64(math.MaxInt64))).
		Set(fields.RuntimeVersion(), builder.Expr("runtime_version + 1")).
		Set(fields.UpdatedAt(), time.Now().UTC()).
		Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("config runtime version")
	}
	return nil
}

var _ SnapshotPublicationRepository = (*SnapshotPublicationRepositoryImpl)(nil)
