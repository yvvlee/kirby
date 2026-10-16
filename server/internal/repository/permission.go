package repository

import (
	"context"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type PermissionIdentity struct {
	SystemAdmin        bool
	EnvironmentID      int64
	EnvironmentVersion int64
	EnvironmentEnabled bool
	EnvironmentMember  bool
}

type PermissionAssignment struct {
	UserID        int64
	EnvironmentID int64
}

type PermissionRepository interface {
	Identity(ctx context.Context, userID, environmentID int64) (PermissionIdentity, error)
	SystemAdmin(ctx context.Context, userID int64) (bool, error)
	KeysForUserEnvironment(ctx context.Context, userID, environmentID int64) ([]string, error)
	List(ctx context.Context) ([]model.Permission, error)
}

type PermissionRepositoryImpl struct {
	*lorm.Repository[*model.Permission]
}

// NewPermissionRepository binds the PermissionRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured PermissionRepository.
// Throws: none.
func NewPermissionRepository(storage *infra.Storage) *PermissionRepositoryImpl {
	return &PermissionRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Permission](),
	}
}

// Identity loads current account status, environment generation, and membership for authorization.
//
// Parameters: ctx controls the reads; userID selects an enabled account; environmentID selects the requested environment.
// Returns: the identity with EnvironmentID zero when the environment is absent, ErrNotFound for an unavailable account, or a database error.
// Throws: none; failures are returned as errors.
func (r *PermissionRepositoryImpl) Identity(ctx context.Context, userID, environmentID int64) (PermissionIdentity, error) {
	user, err := r.enabledUser(ctx, userID)
	if err != nil {
		return PermissionIdentity{}, err
	}
	identity := PermissionIdentity{SystemAdmin: user.IsSystemAdmin}
	if environmentID <= 0 {
		return identity, nil
	}
	environment, found, err := r.Engine.Query[*model.Environment]().ID(environmentID).
		Where(builder.IsNull(new(model.Environment).LormCols().DeletedAt())).Get(ctx)
	if err != nil {
		return PermissionIdentity{}, WrapDatabaseError(err)
	}
	if found {
		identity.EnvironmentID = environment.ID
		identity.EnvironmentVersion = environment.Version
		identity.EnvironmentEnabled = environment.Enabled
	}
	fields := new(model.UserEnvironmentRole).LormCols()
	identity.EnvironmentMember, err = r.Engine.Query[*model.UserEnvironmentRole]().
		Where(builder.Eq{
			fields.UserID():        userID,
			fields.EnvironmentID(): environmentID,
		}).Exist(ctx)
	if err != nil {
		return PermissionIdentity{}, WrapDatabaseError(err)
	}
	return identity, nil
}

// SystemAdmin checks persisted enabled-account status instead of trusting token claims.
//
// Parameters: ctx controls the read; userID identifies the account.
// Returns: whether the live enabled account is an administrator, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *PermissionRepositoryImpl) SystemAdmin(ctx context.Context, userID int64) (bool, error) {
	user, err := r.enabledUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.IsSystemAdmin, nil
}

func (r *PermissionRepositoryImpl) enabledUser(ctx context.Context, userID int64) (*model.User, error) {
	fields := new(model.User).LormCols()
	user, found, err := r.Engine.Query[*model.User]().ID(userID).
		Where(builder.And{
			builder.Eq{
				fields.Enabled(): true,
			},
			builder.IsNull(fields.DeletedAt()),
		}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("permission identity")
	}
	return user, nil
}

// KeysForUserEnvironment excludes deleted roles and duplicate grants from environment authorization.
//
// Parameters: ctx controls the read; userID and environmentID select the role assignments.
// Returns: sorted unique permission keys, or a database error.
// Throws: none; failures are returned as errors.
func (r *PermissionRepositoryImpl) KeysForUserEnvironment(ctx context.Context, userID, environmentID int64) ([]string, error) {
	keys, err := r.Engine.Query[*model.Permission]().From("permissions p").Select("p.key").
		InnerJoin("role_permissions rp ON rp.permission_id = p.id").
		InnerJoin("user_environment_roles uer ON uer.role_id = rp.role_id").
		InnerJoin("roles r ON r.id = rp.role_id AND r.deleted_at IS NULL").
		Where(builder.Eq{
			"uer.user_id":        userID,
			"uer.environment_id": environmentID,
		}).Distinct().Asc("p.key").FindCols[string](ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return keys, nil
}

// List returns the complete permission catalog in stable ID order.
//
// Parameters: ctx controls the database read.
// Returns: catalog entries ordered by ID, or a database error.
// Throws: none; failures are returned as errors.
func (r *PermissionRepositoryImpl) List(ctx context.Context) ([]model.Permission, error) {
	permissions, err := r.Engine.Query[*model.Permission]().Asc(new(model.Permission).LormCols().ID()).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(permissions), nil
}

var _ PermissionRepository = (*PermissionRepositoryImpl)(nil)
