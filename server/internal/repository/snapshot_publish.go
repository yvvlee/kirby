package repository

import (
	"context"
	"time"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

// SnapshotPublicationRepository contains the guarded writes used only by the
// direct publish workflow. Callers must lock the owning config, structures and
// enums before LockForConfig so every config workflow takes locks in one order.
type SnapshotPublicationRepository struct {
	engine *xorm.Engine
}

func NewSnapshotPublicationRepository(engine *xorm.Engine) *SnapshotPublicationRepository {
	return &SnapshotPublicationRepository{engine: engine}
}

func (r *SnapshotPublicationRepository) LockForConfig(ctx context.Context, tx *xorm.Session, environmentID, configID int64) ([]model.Snapshot, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, base.InvalidArgument("transaction session is nil")
	}
	items := make([]model.Snapshot, 0)
	if err := tx.Context(ctx).SQL(lockSnapshotsForConfigSQL, environmentID, configID).Find(&items); err != nil {
		return nil, base.Wrap("lock snapshots for config", err)
	}
	return items, nil
}

const lockSnapshotsForConfigSQL = `
SELECT s.*
FROM snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND c.id = ? AND s.deleted_at IS NULL
ORDER BY s.id ASC
FOR UPDATE`

func (r *SnapshotPublicationRepository) SetReleased(ctx context.Context, tx *xorm.Session, environmentID, configID, snapshotID, expectedVersion, updatedBy int64, publishedAt time.Time) error {
	if err := validatePublicationWrite(environmentID, configID, snapshotID, expectedVersion, updatedBy); err != nil {
		return err
	}
	if publishedAt.IsZero() {
		return base.InvalidArgument("published_at is zero")
	}
	_, err := base.ExecuteTx(ctx, tx, "snapshot publication", `
UPDATE snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
SET s.status = ?, s.published_at = ?, s.published_by = ?, s.updated_by = ?,
    s.updated_at = ?, s.version = s.version + 1
WHERE e.id = ? AND c.id = ? AND s.id = ?
  AND s.status = ? AND s.version = ? AND s.deleted_at IS NULL`,
		model.SnapshotStatusReleased, publishedAt, updatedBy, updatedBy, publishedAt,
		environmentID, configID, snapshotID, model.SnapshotStatusUnreleased, expectedVersion)
	return err
}

func (r *SnapshotPublicationRepository) SetUnreleased(ctx context.Context, tx *xorm.Session, environmentID, configID, snapshotID, expectedVersion, updatedBy int64, updatedAt time.Time) error {
	if err := validatePublicationWrite(environmentID, configID, snapshotID, expectedVersion, updatedBy); err != nil {
		return err
	}
	if updatedAt.IsZero() {
		return base.InvalidArgument("updated_at is zero")
	}
	_, err := base.ExecuteTx(ctx, tx, "snapshot unpublication", `
UPDATE snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
SET s.status = ?, s.published_at = NULL, s.published_by = NULL, s.updated_by = ?,
    s.updated_at = ?, s.version = s.version + 1
WHERE e.id = ? AND c.id = ? AND s.id = ?
  AND s.status = ? AND s.version = ? AND s.deleted_at IS NULL`,
		model.SnapshotStatusUnreleased, updatedBy, updatedAt, environmentID, configID,
		snapshotID, model.SnapshotStatusReleased, expectedVersion)
	return err
}

func (r *SnapshotPublicationRepository) IncrementRuntimeVersion(ctx context.Context, tx *xorm.Session, environmentID, configID int64) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "config runtime version", `
UPDATE configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
SET c.runtime_version = c.runtime_version + 1
WHERE e.id = ? AND c.id = ? AND c.deleted_at IS NULL
  AND c.runtime_version < 9223372036854775807`, environmentID, configID)
	return err
}

func validatePublicationWrite(environmentID, configID, snapshotID, expectedVersion, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	if err := base.ValidateID("snapshot_id", snapshotID); err != nil {
		return err
	}
	if expectedVersion < 0 {
		return base.InvalidArgument("version must not be negative")
	}
	return base.ValidateID("updated_by", updatedBy)
}
