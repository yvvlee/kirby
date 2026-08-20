package repository

import (
	"context"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type SnapshotFilter struct {
	ProjectID int64
	ConfigID  int64
	Status    *model.SnapshotStatus
	IsUsing   *bool
}

type SnapshotRepository interface {
	Create(context.Context, int64, int64, int64, *model.Snapshot) error
	CreateTx(context.Context, *xorm.Session, int64, int64, int64, *model.Snapshot) error
	FindByID(context.Context, int64, int64) (*model.Snapshot, error)
	List(context.Context, int64, SnapshotFilter, base.PageRequest) (base.PageResult[model.Snapshot], error)
	FindReleasedForConfig(context.Context, int64, int64) (*model.Snapshot, error)
	FindReleasedForConfigTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	FindCurrentForConfig(context.Context, int64, int64) (*model.Snapshot, error)
	FindCurrentForConfigTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	FindAnyForConfigTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	ListReleasedConfigIDs(context.Context, int64, int64) ([]int64, error)
	Delete(context.Context, int64, int64, int64) error
	DeleteTx(context.Context, *xorm.Session, int64, int64, int64) error
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	SetCurrent(context.Context, *xorm.Session, int64, int64, int64, int64) error
}

type SnapshotRepositoryImpl struct {
	engine *xorm.Engine
}

func NewSnapshotRepository(engine *xorm.Engine) *SnapshotRepositoryImpl {
	return &SnapshotRepositoryImpl{engine: engine}
}

func (r *SnapshotRepositoryImpl) Create(ctx context.Context, environmentID, projectID, configID int64, snapshot *model.Snapshot) error {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return err
	}
	if err := base.ValidateID("config_id", configID); err != nil {
		return err
	}
	if snapshot == nil {
		return base.InvalidArgument("snapshot is nil")
	}
	if snapshot.ProjectID != 0 && snapshot.ProjectID != projectID {
		return base.InvalidArgument("snapshot.project_id does not match project_id")
	}
	if snapshot.ConfigID != 0 && snapshot.ConfigID != configID {
		return base.InvalidArgument("snapshot.config_id does not match config_id")
	}
	snapshot.ProjectID = projectID
	snapshot.ConfigID = configID
	snapshot.Status = model.SnapshotStatusUnreleased
	snapshot.IsUsing = false
	result, err := base.Execute(ctx, r.engine, "snapshot", insertSnapshotSQL,
		snapshotCreateArgs(environmentID, projectID, configID, snapshot)...)
	if err != nil {
		return err
	}
	snapshot.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted snapshot id", err)
	}
	return nil
}

func (r *SnapshotRepositoryImpl) CreateTx(ctx context.Context, tx *xorm.Session, environmentID, projectID, configID int64, snapshot *model.Snapshot) error {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return err
	}
	if err := base.ValidateID("config_id", configID); err != nil {
		return err
	}
	if snapshot == nil {
		return base.InvalidArgument("snapshot is nil")
	}
	if snapshot.ProjectID != 0 && snapshot.ProjectID != projectID {
		return base.InvalidArgument("snapshot.project_id does not match project_id")
	}
	if snapshot.ConfigID != 0 && snapshot.ConfigID != configID {
		return base.InvalidArgument("snapshot.config_id does not match config_id")
	}
	snapshot.ProjectID = projectID
	snapshot.ConfigID = configID
	snapshot.Status = model.SnapshotStatusUnreleased
	snapshot.IsUsing = false
	result, err := base.ExecuteTx(ctx, tx, "snapshot", insertSnapshotSQL,
		snapshotCreateArgs(environmentID, projectID, configID, snapshot)...)
	if err != nil {
		return err
	}
	snapshot.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted snapshot id", err)
	}
	return nil
}

const insertSnapshotSQL = `
INSERT INTO snapshots
    (project_id, config_id, config_key, description, content, status, tags_json,
     is_using, created_by, updated_by)
SELECT p.id, c.id, c.` + "`key`" + `, ?, ?, ?, ?, FALSE, ?, ?
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE p.id = ? AND e.id = ? AND c.id = ? AND c.deleted_at IS NULL`

func snapshotCreateArgs(environmentID, projectID, configID int64, snapshot *model.Snapshot) []any {
	return []any{
		snapshot.Description, snapshot.Content, snapshot.Status, snapshot.TagsJSON,
		snapshot.CreatedBy, snapshot.UpdatedBy, projectID, environmentID, configID,
	}
}

func (r *SnapshotRepositoryImpl) FindByID(ctx context.Context, environmentID, snapshotID int64) (*model.Snapshot, error) {
	if err := validateEnvironmentResource(environmentID, "snapshot_id", snapshotID); err != nil {
		return nil, err
	}
	var snapshot model.Snapshot
	err := base.FindOne(ctx, r.engine, "snapshot", snapshotByIDSQL, []any{environmentID, snapshotID}, &snapshot)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *SnapshotRepositoryImpl) List(ctx context.Context, environmentID int64, filter SnapshotFilter, page base.PageRequest) (base.PageResult[model.Snapshot], error) {
	if err := validateEnvironmentResource(environmentID, "project_id", filter.ProjectID); err != nil {
		return base.PageResult[model.Snapshot]{}, err
	}
	if err := base.ValidateID("config_id", filter.ConfigID); err != nil {
		return base.PageResult[model.Snapshot]{}, err
	}
	where := "e.id = ? AND p.id = ? AND c.id = ? AND s.deleted_at IS NULL"
	args := []any{environmentID, filter.ProjectID, filter.ConfigID}
	if filter.Status != nil {
		if !validSnapshotStatus(*filter.Status) {
			return base.PageResult[model.Snapshot]{}, base.InvalidArgument("invalid snapshot status")
		}
		where += " AND s.status = ?"
		args = append(args, *filter.Status)
	}
	if filter.IsUsing != nil {
		where += " AND s.is_using = ?"
		args = append(args, *filter.IsUsing)
	}
	page = base.NormalizePage(page)
	from := `
FROM snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE ` + where
	total, err := base.Count(ctx, r.engine, "snapshots", "SELECT COUNT(*) AS total"+from, args...)
	if err != nil {
		return base.PageResult[model.Snapshot]{}, err
	}
	result := base.PageResult[model.Snapshot]{Items: []model.Snapshot{}, Total: total, Offset: page.Offset, Limit: page.Limit}
	if total == 0 {
		return result, nil
	}
	listArgs := append(append([]any{}, args...), page.Limit, page.Offset)
	err = base.FindAll(ctx, r.engine, "snapshots", "SELECT s.*"+from+`
ORDER BY s.id DESC
LIMIT ? OFFSET ?`, listArgs, &result.Items)
	return result, err
}

func (r *SnapshotRepositoryImpl) FindReleasedForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfig(ctx, environmentID, configID, "s.status = ?", model.SnapshotStatusReleased, "released snapshot")
}

func (r *SnapshotRepositoryImpl) FindCurrentForConfig(ctx context.Context, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfig(ctx, environmentID, configID, "s.is_using = TRUE", nil, "current snapshot")
}

func (r *SnapshotRepositoryImpl) FindReleasedForConfigTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfigTx(ctx, tx, environmentID, configID, "s.status = ?", model.SnapshotStatusReleased, "released snapshot")
}

func (r *SnapshotRepositoryImpl) FindCurrentForConfigTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfigTx(ctx, tx, environmentID, configID, "s.is_using = TRUE", nil, "current snapshot")
}

func (r *SnapshotRepositoryImpl) FindAnyForConfigTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64) (*model.Snapshot, error) {
	return r.findForConfigTx(ctx, tx, environmentID, configID, "1 = 1", nil, "snapshot")
}

func (r *SnapshotRepositoryImpl) findForConfig(ctx context.Context, environmentID, configID int64, condition string, argument any, resource string) (*model.Snapshot, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	args := []any{environmentID, configID}
	if argument != nil {
		args = append(args, argument)
	}
	var snapshot model.Snapshot
	err := base.FindOne(ctx, r.engine, resource, `
SELECT s.*
FROM snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND c.id = ? AND s.deleted_at IS NULL AND `+condition+`
ORDER BY s.id DESC
LIMIT 1`, args, &snapshot)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *SnapshotRepositoryImpl) findForConfigTx(ctx context.Context, tx *xorm.Session, environmentID, configID int64, condition string, argument any, resource string) (*model.Snapshot, error) {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return nil, err
	}
	args := []any{environmentID, configID}
	if argument != nil {
		args = append(args, argument)
	}
	var snapshot model.Snapshot
	err := base.LockOne(ctx, tx, resource, `
SELECT s.*
FROM snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND c.id = ? AND s.deleted_at IS NULL AND `+condition+`
ORDER BY s.id DESC
LIMIT 1
FOR UPDATE`, args, &snapshot)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *SnapshotRepositoryImpl) ListReleasedConfigIDs(ctx context.Context, environmentID, projectID int64) ([]int64, error) {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return nil, err
	}
	rows := make([]struct {
		ConfigID int64 `xorm:"config_id"`
	}, 0)
	err := base.FindAll(ctx, r.engine, "released config ids", `
SELECT DISTINCT s.config_id
FROM snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND p.id = ? AND s.status = ? AND s.deleted_at IS NULL
ORDER BY s.config_id ASC`, []any{environmentID, projectID, model.SnapshotStatusReleased}, &rows)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(rows))
	for i := range rows {
		ids[i] = rows[i].ConfigID
	}
	return ids, nil
}

func (r *SnapshotRepositoryImpl) Delete(ctx context.Context, environmentID, snapshotID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "snapshot_id", snapshotID); err != nil {
		return err
	}
	_, err := base.Execute(ctx, r.engine, "snapshot", `
UPDATE snapshots AS s
SET s.deleted_at = UTC_TIMESTAMP(6), s.updated_by = ?,
    s.updated_at = UTC_TIMESTAMP(6), s.version = s.version + 1
WHERE s.id = ? AND s.status = ? AND s.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
      WHERE p.id = s.project_id AND e.id = ? AND p.deleted_at IS NULL
  )`, updatedBy, snapshotID, model.SnapshotStatusUnreleased, environmentID)
	return err
}

func (r *SnapshotRepositoryImpl) DeleteTx(ctx context.Context, tx *xorm.Session, environmentID, snapshotID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "snapshot_id", snapshotID); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "snapshot", `
UPDATE snapshots AS s
SET s.deleted_at = UTC_TIMESTAMP(6), s.updated_by = ?,
    s.updated_at = UTC_TIMESTAMP(6), s.version = s.version + 1
WHERE s.id = ? AND s.status = ? AND s.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM projects AS p
      INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
      WHERE p.id = s.project_id AND e.id = ? AND p.deleted_at IS NULL
  )`, updatedBy, snapshotID, model.SnapshotStatusUnreleased, environmentID)
	return err
}

func (r *SnapshotRepositoryImpl) LockByID(ctx context.Context, tx *xorm.Session, environmentID, snapshotID int64) (*model.Snapshot, error) {
	if err := validateEnvironmentResource(environmentID, "snapshot_id", snapshotID); err != nil {
		return nil, err
	}
	var snapshot model.Snapshot
	err := base.LockOne(ctx, tx, "snapshot", snapshotByIDSQL+"\nFOR UPDATE", []any{environmentID, snapshotID}, &snapshot)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *SnapshotRepositoryImpl) SetCurrent(ctx context.Context, tx *xorm.Session, environmentID, configID, snapshotID, updatedBy int64) error {
	if err := validateEnvironmentResource(environmentID, "config_id", configID); err != nil {
		return err
	}
	if err := base.ValidateID("snapshot_id", snapshotID); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "current snapshot", `
UPDATE snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
INNER JOIN snapshots AS target
    ON target.id = ? AND target.config_id = c.id AND target.project_id = p.id
   AND target.deleted_at IS NULL
SET s.is_using = (s.id = ?), s.updated_by = ?,
    s.updated_at = UTC_TIMESTAMP(6), s.version = s.version + 1
WHERE e.id = ? AND c.id = ? AND s.deleted_at IS NULL`,
		snapshotID, snapshotID, updatedBy, environmentID, configID)
	return err
}

const snapshotByIDSQL = `
SELECT s.*
FROM snapshots AS s
INNER JOIN configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL
INNER JOIN projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND s.id = ? AND s.deleted_at IS NULL
LIMIT 1`

func validSnapshotStatus(status model.SnapshotStatus) bool {
	return status == model.SnapshotStatusUnreleased || status == model.SnapshotStatusReleased
}

var _ SnapshotRepository = (*SnapshotRepositoryImpl)(nil)
