package repository

import (
	"context"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type EnvironmentPermissionRepository interface {
	LockScope(ctx context.Context, environmentID int64) error
	List(ctx context.Context, environmentID int64) ([]model.EnvironmentPermissionGrant, error)
	Replace(ctx context.Context, environmentID int64, role string, permissions []string, actorID int64) error
}

type EnvironmentPermissionRepositoryImpl struct {
	*lorm.Repository[*model.EnvironmentPermissionGrant]
}

// NewEnvironmentPermissionRepository enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func NewEnvironmentPermissionRepository(storage *infra.Storage) *EnvironmentPermissionRepositoryImpl {
	return &EnvironmentPermissionRepositoryImpl{Repository: storage.CoreDB.Repository[*model.EnvironmentPermissionGrant]()}
}

// List enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func (r *EnvironmentPermissionRepositoryImpl) List(ctx context.Context, environmentID int64) ([]model.EnvironmentPermissionGrant, error) {
	f := new(model.EnvironmentPermissionGrant).LormCols()
	items, err := r.Engine.Query[*model.EnvironmentPermissionGrant]().Where(builder.Eq{f.EnvironmentID(): environmentID}).Asc(f.ID()).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(items), nil
}

// Replace enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func (r *EnvironmentPermissionRepositoryImpl) Replace(ctx context.Context, environmentID int64, role string, permissions []string, actorID int64) error {
	f := new(model.EnvironmentPermissionGrant).LormCols()
	if _, err := r.Engine.Delete[*model.EnvironmentPermissionGrant]().Where(builder.Eq{
		f.EnvironmentID(): environmentID,
		f.Role():          role,
	}).Exec(ctx); err != nil {
		return WrapDatabaseError(err)
	}
	for _, permission := range permissions {
		_, err := r.Insert(ctx, &model.EnvironmentPermissionGrant{
			EnvironmentID: environmentID,
			Role:          role,
			Permission:    permission,
			CreatedBy:     actorID,
		})
		if err != nil {
			return WrapDatabaseError(err)
		}
	}
	_, err := r.Engine.Update[*model.Environment]().ID(environmentID).Set("version", builder.Expr("version + 1")).Exec(ctx)
	return WrapDatabaseError(err)
}

var _ EnvironmentPermissionRepository = (*EnvironmentPermissionRepositoryImpl)(nil)

// LockScope serializes matrix changes with project membership changes.
// Parameters: ctx carries the transaction; environmentID selects the scope.
// Returns: a persistence error or nil after both locks.
// Throws: none; errors are returned.
// LockScope enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func (r *EnvironmentPermissionRepositoryImpl) LockScope(ctx context.Context, environmentID int64) error {
	env, found, err := r.Engine.Query[*model.Environment]().ID(environmentID).Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !found {
		return Missing("environment")
	}
	_, found, err = r.Engine.Query[*model.Project]().ID(env.ProjectID).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !found {
		return Missing("project")
	}
	_, found, err = r.Engine.Query[*model.Environment]().ID(environmentID).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !found {
		return Missing("environment")
	}
	return nil
}
