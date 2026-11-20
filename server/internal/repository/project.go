package repository

import (
	"context"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type ProjectUpdate struct {
	Name        string
	Description string
	UpdatedBy   int64
	Version     int64
}

type ProjectRepository interface {
	Delete(ctx context.Context, projectID int64) error
	Create(ctx context.Context, environmentID int64, project *model.Project) (rowsAffected int64, err error)
	FindByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error)
	List(ctx context.Context, environmentID int64, keyword string, page PageRequest) (PageResult[model.Project], error)
	Update(ctx context.Context, environmentID, projectID int64, update ProjectUpdate) (rowsAffected int64, err error)
	LockByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error)
}

type ProjectRepositoryImpl struct {
	*lorm.Repository[*model.Project]
}

// NewProjectRepository creates ProjectRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *ProjectRepositoryImpl.
// Throws: none.
func NewProjectRepository(storage *infra.Storage) *ProjectRepositoryImpl {
	return &ProjectRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Project](),
	}
}

// Create inserts a system-level project after the caller has checked project-management permission.
//
// Parameters: ctx carries cancellation and transaction state; project contains the new project metadata.
// Returns: rowsAffected from the project insert with its ID, version, and timestamps set; database failures are returned.
// Throws: none.
func (r *ProjectRepositoryImpl) Create(ctx context.Context, _ int64, project *model.Project) (rowsAffected int64, err error) {
	rowsAffected, err = r.Insert(ctx, project)
	if err != nil {
		err = WrapDatabaseError(err)
	}
	return
}

// FindByID loads a resource only when it belongs to the requested scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; projectID selects the project.
// Returns: the live row, or ErrNotFound when the ID is missing, deleted, or outside the environment.
// Throws: none.
func (r *ProjectRepositoryImpl) FindByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error) {
	var query *lorm.SelectStmt[*model.Project]
	if environmentID > 0 {
		query = projectScope(r.Engine).Where(builder.Eq{
			"e.id": environmentID,
			"p.id": projectID,
		})
	} else {
		query = r.Engine.Query[*model.Project]().ID(projectID)
	}
	project, found, err := query.Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("project")
	}
	return project, nil
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; keyword matches project names or descriptions; page contains pagination bounds.
// Returns: the scoped page, total row count, normalized offset and limit, or a database error.
// Throws: none.
func (r *ProjectRepositoryImpl) List(ctx context.Context, environmentID int64, keyword string, page PageRequest) (PageResult[model.Project], error) {
	pattern := "%" + keyword + "%"
	condition := builder.Or{
		builder.Like("p.name", pattern),
		builder.Like("p.description", pattern),
	}
	query := r.Engine.Query[*model.Project]().Select("p.*").From("projects AS p")
	if environmentID > 0 {
		query = projectScope(r.Engine).Where(builder.Eq{"e.id": environmentID})
	}
	return paginate[model.Project](ctx, query.Where(condition).Desc("p.id"), page)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; projectID selects the project; update contains the guarded changes and expected version.
// Returns: rowsAffected after one matching live row changes; ErrNoRowsAffected if the expected version is stale.
// Throws: none.
func (r *ProjectRepositoryImpl) Update(ctx context.Context, _, projectID int64, update ProjectUpdate) (rowsAffected int64, err error) {
	fields := new(model.Project).LormCols()
	rowsAffected, err = r.Engine.Update[*model.Project]().ID(projectID).
		Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Name():        update.Name,
			fields.Description(): update.Description,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("project")
		return
	}
	return
}

// LockByID locks a scoped resource so concurrent writes share the same order.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; projectID selects the project.
// Returns: the locked live row, or ErrNotFound for a missing or inaccessible resource; requires the caller’s transaction.
// Throws: none.
func (r *ProjectRepositoryImpl) LockByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error) {
	query := r.Engine.Query[*model.Project]().ID(projectID)
	if environmentID > 0 {
		query = projectScope(r.Engine).Where(builder.Eq{
			"e.id": environmentID,
			"p.id": projectID,
		})
	}
	project, has, err := query.Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, Missing("project")
	}
	return project, nil
}

var _ ProjectRepository = (*ProjectRepositoryImpl)(nil)

// Delete removes a project graph while retaining immutable audit records.
// Parameters: ctx carries the project-locked transaction; projectID selects its graph.
// Returns: nil or the first persistence error.
// Throws: none; errors are returned.
func (r *ProjectRepositoryImpl) Delete(ctx context.Context, projectID int64) error {
	configs, err := r.Engine.Query[*model.Config]().Where(builder.Eq{"project_id": projectID}).Find(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	for _, config := range configs {
		if err := r.deleteConfigChildren(ctx, config.ID); err != nil {
			return err
		}
	}
	environments, err := r.Engine.Query[*model.Environment]().Where(builder.Eq{"project_id": projectID}).Find(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	for _, env := range environments {
		if _, err := r.Engine.Delete[*model.ImportRecord]().Where(builder.Or{builder.Eq{"source_environment_id": env.ID}, builder.Eq{"target_environment_id": env.ID}}).Exec(ctx); err != nil {
			return WrapDatabaseError(err)
		}
		if _, err := r.Engine.Delete[*model.EnvironmentPermissionGrant]().Where(builder.Eq{"environment_id": env.ID}).Exec(ctx); err != nil {
			return WrapDatabaseError(err)
		}
	}
	return r.deleteProjectRows(ctx, projectID)
}
func (r *ProjectRepositoryImpl) deleteConfigChildren(ctx context.Context, configID int64) error {
	if _, err := r.Engine.Delete[*model.Structure]().Where(builder.Eq{"config_id": configID}).Exec(ctx); err != nil {
		return WrapDatabaseError(err)
	}
	if _, err := r.Engine.Delete[*model.ConfigEnum]().Where(builder.Eq{"config_id": configID}).Exec(ctx); err != nil {
		return WrapDatabaseError(err)
	}
	return nil
}
func (r *ProjectRepositoryImpl) deleteProjectRows(ctx context.Context, projectID int64) error {
	steps := []func() error{
		func() error {
			_, err := r.Engine.Delete[*model.Snapshot]().Where(builder.Eq{"project_id": projectID}).Exec(ctx)
			return WrapDatabaseError(err)
		},
		func() error {
			_, err := r.Engine.Delete[*model.Config]().Where(builder.Eq{"project_id": projectID}).Exec(ctx)
			return WrapDatabaseError(err)
		},
		func() error {
			_, err := r.Engine.Delete[*model.ProjectAPIKey]().Where(builder.Eq{"project_id": projectID}).Exec(ctx)
			return WrapDatabaseError(err)
		},
		func() error {
			_, err := r.Engine.Delete[*model.ProjectInvitation]().Where(builder.Eq{"project_id": projectID}).Exec(ctx)
			return WrapDatabaseError(err)
		},
		func() error {
			_, err := r.Engine.Delete[*model.ImportRecord]().Where(builder.Eq{"target_project_id": projectID}).Exec(ctx)
			return WrapDatabaseError(err)
		},
		func() error {
			_, err := r.Engine.Delete[*model.Environment]().Where(builder.Eq{"project_id": projectID}).Exec(ctx)
			return WrapDatabaseError(err)
		},
		func() error {
			_, err := r.Engine.Delete[*model.ProjectMember]().Where(builder.Eq{"project_id": projectID}).Exec(ctx)
			return WrapDatabaseError(err)
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	_, err := r.Engine.Delete[*model.Project]().ID(projectID).Exec(ctx)
	return WrapDatabaseError(err)
}
