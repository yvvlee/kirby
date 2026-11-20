package repository

import (
	"context"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

// ProjectMemberRepository persists project membership and fixed roles.
type ProjectMemberRepository interface {
	GetForUpdate(ctx context.Context, projectID, userID int64) (*model.ProjectMember, error)
	Add(ctx context.Context, member *model.ProjectMember) error
	IsMember(ctx context.Context, projectID, userID int64) (bool, error)
	ListProjectIDs(ctx context.Context, userID int64) ([]int64, error)
	Get(ctx context.Context, projectID, userID int64) (*model.ProjectMember, error)
	List(ctx context.Context, projectID int64) ([]model.ProjectMember, error)
	UpdateRole(ctx context.Context, projectID, userID int64, role string) error
	Remove(ctx context.Context, projectID, userID int64) error
	CountOwners(ctx context.Context, projectID int64) (int64, error)
}

// Get maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) Get(ctx context.Context, projectID, userID int64) (*model.ProjectMember, error) {
	f := new(model.ProjectMember).LormCols()
	item, found, err := r.Engine.Query[*model.ProjectMember]().Where(builder.Eq{
		f.ProjectID(): projectID,
		f.UserID():    userID,
	}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("project member")
	}
	return item, nil
}

// List maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) List(ctx context.Context, projectID int64) ([]model.ProjectMember, error) {
	f := new(model.ProjectMember).LormCols()
	items, err := r.Engine.Query[*model.ProjectMember]().Where(builder.Eq{f.ProjectID(): projectID}).Asc(f.ID()).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(items), nil
}

// UpdateRole maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) UpdateRole(ctx context.Context, projectID, userID int64, role string) error {
	f := new(model.ProjectMember).LormCols()
	rows, err := r.Engine.Update[*model.ProjectMember]().Where(builder.Eq{
		f.ProjectID(): projectID,
		f.UserID():    userID,
	}).SetMap(map[string]any{f.Role(): role}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if rows == 0 {
		return Missing("project member")
	}
	return nil
}

// Remove maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) Remove(ctx context.Context, projectID, userID int64) error {
	f := new(model.ProjectMember).LormCols()
	rows, err := r.Engine.Delete[*model.ProjectMember]().Where(builder.Eq{
		f.ProjectID(): projectID,
		f.UserID():    userID,
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if rows == 0 {
		return Missing("project member")
	}
	return nil
}

// CountOwners maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) CountOwners(ctx context.Context, projectID int64) (int64, error) {
	f := new(model.ProjectMember).LormCols()
	count, err := r.Engine.Query[*model.ProjectMember]().Where(builder.Eq{
		f.ProjectID(): projectID,
		f.Role():      "OWNER",
	}).Count(ctx)
	if err != nil {
		return 0, WrapDatabaseError(err)
	}
	return int64(count), nil
}

type ProjectMemberRepositoryImpl struct {
	*lorm.Repository[*model.ProjectMember]
}

// NewProjectMemberRepository creates the project membership repository.
// NewProjectMemberRepository maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func NewProjectMemberRepository(storage *infra.Storage) *ProjectMemberRepositoryImpl {
	return &ProjectMemberRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.ProjectMember](),
	}
}

// Add maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) Add(ctx context.Context, member *model.ProjectMember) error {
	if _, err := r.Insert(ctx, member); err != nil {
		return WrapDatabaseError(err)
	}
	return nil
}

// IsMember maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) IsMember(ctx context.Context, projectID, userID int64) (bool, error) {
	fields := new(model.ProjectMember).LormCols()
	found, err := r.Engine.Query[*model.ProjectMember]().Where(builder.Eq{
		fields.ProjectID(): projectID,
		fields.UserID():    userID,
	}).Exist(ctx)
	if err != nil {
		return false, WrapDatabaseError(err)
	}
	return found, nil
}

// ListProjectIDs maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (r *ProjectMemberRepositoryImpl) ListProjectIDs(ctx context.Context, userID int64) ([]int64, error) {
	fields := new(model.ProjectMember).LormCols()
	ids, err := r.Engine.Query[*model.ProjectMember]().Select(fields.ProjectID()).Where(builder.Eq{
		fields.UserID(): userID,
	}).Asc(fields.ProjectID()).FindCols[int64](ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return ids, nil
}

var _ ProjectMemberRepository = (*ProjectMemberRepositoryImpl)(nil)

// GetForUpdate reads current membership after the project lock is acquired.
// Parameters: ctx carries a transaction; projectID and userID identify membership.
// Returns: the current locked membership.
// Throws: missing membership or database errors.
func (r *ProjectMemberRepositoryImpl) GetForUpdate(ctx context.Context, projectID, userID int64) (*model.ProjectMember, error) {
	f := new(model.ProjectMember).LormCols()
	item, found, err := r.Engine.Query[*model.ProjectMember]().Where(builder.Eq{
		f.ProjectID(): projectID,
		f.UserID():    userID,
	}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("project member")
	}
	return item, nil
}
