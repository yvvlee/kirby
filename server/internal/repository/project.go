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
	Create(ctx context.Context, environmentID int64, project *model.Project) error
	FindByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error)
	List(ctx context.Context, environmentID int64, keyword string, page PageRequest) (PageResult[model.Project], error)
	Update(ctx context.Context, environmentID, projectID int64, update ProjectUpdate) error
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
// Returns: nil after inserting the project and setting its ID, version, and timestamps; database failures are returned.
// Throws: none.
func (r *ProjectRepositoryImpl) Create(ctx context.Context, _ int64, project *model.Project) error {
	project.PrepareInsert()
	_, err := r.Insert(ctx, project)
	return WrapDatabaseError(err)
}

// FindByID loads a resource only when it belongs to the requested scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; projectID selects the project.
// Returns: the live row, or ErrNotFound when the ID is missing, deleted, or outside the environment.
// Throws: none.
func (r *ProjectRepositoryImpl) FindByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error) {
	fields := new(model.Project).LormCols()
	query := r.Engine.Query[*model.Project]().ID(projectID).Where(builder.IsNull(fields.DeletedAt()))
	if environmentID > 0 {
		query = projectScope(r.Engine).Where(builder.Eq{
			"e.id": environmentID,
			"p.id": projectID,
		})
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
	query := r.Engine.Query[*model.Project]().Select("p.*").From("projects AS p").
		Where(builder.IsNull("p.deleted_at"))
	if environmentID > 0 {
		query = projectScope(r.Engine).Where(builder.Eq{"e.id": environmentID})
	}
	return paginate[model.Project](ctx, query.Where(condition).Desc("p.id"), page)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; projectID selects the project; update contains the guarded changes and expected version.
// Returns: nil after one matching live row changes; ErrNoRowsAffected if the expected version is stale.
// Throws: none.
func (r *ProjectRepositoryImpl) Update(ctx context.Context, _, projectID int64, update ProjectUpdate) error {
	fields := new(model.Project).LormCols()
	affected, err := r.Engine.Update[*model.Project]().Where(builder.IsNull(fields.DeletedAt())).ID(projectID).
		Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Name():        update.Name,
			fields.Description(): update.Description,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now().UTC(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("project")
	}
	return nil
}

// LockByID locks a scoped resource so concurrent writes share the same order.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; projectID selects the project.
// Returns: the locked live row, or ErrNotFound for a missing or inaccessible resource; requires the caller’s transaction.
// Throws: none.
func (r *ProjectRepositoryImpl) LockByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error) {
	project, has, err := projectScope(r.Engine).
		Where(builder.Eq{
			"e.id": environmentID,
			"p.id": projectID,
		}).
		Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !has {
		return nil, Missing("project")
	}
	return project, nil
}

var _ ProjectRepository = (*ProjectRepositoryImpl)(nil)
