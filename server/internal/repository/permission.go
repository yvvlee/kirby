package repository

import (
	"context"
	"sort"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type PermissionIdentity struct {
	ProjectRole        string
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
}

type PermissionRepositoryImpl struct {
	*lorm.Repository[*model.ProjectMember]
}

// NewPermissionRepository binds the PermissionRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured PermissionRepository.
// Throws: none.
func NewPermissionRepository(storage *infra.Storage) *PermissionRepositoryImpl {
	return &PermissionRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.ProjectMember](),
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
	environment, found, err := r.Engine.Query[*model.Environment]().ID(environmentID).Get(ctx)
	if err != nil {
		return PermissionIdentity{}, WrapDatabaseError(err)
	}
	if found {
		identity.EnvironmentID = environment.ID
		identity.EnvironmentVersion = environment.Version
		identity.EnvironmentEnabled = environment.Enabled
	}
	member, memberFound, err := r.Engine.Query[*model.ProjectMember]().From("project_members pm").Select("pm.*").
		InnerJoin("environments e ON e.project_id = pm.project_id").
		Where(builder.Eq{
			"pm.user_id": userID,
			"e.id":       environmentID,
		}).Get(ctx)
	if err != nil {
		return PermissionIdentity{}, WrapDatabaseError(err)
	}
	identity.EnvironmentMember = memberFound
	if memberFound {
		identity.ProjectRole = member.Role
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
		Where(builder.Eq{
			fields.Enabled(): true,
		}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("permission identity")
	}
	return user, nil
}

// KeysForUserEnvironment excludes duplicate grants from environment authorization.
//
// Parameters: ctx controls the read; userID and environmentID select the role assignments.
// Returns: sorted unique permission keys, or a database error.
// Throws: none; failures are returned as errors.
func (r *PermissionRepositoryImpl) KeysForUserEnvironment(ctx context.Context, userID, environmentID int64) ([]string, error) {
	grants, err := r.Engine.Query[*model.EnvironmentPermissionGrant]().From("environment_permission_grants epg").Select("epg.*").
		InnerJoin("environments e ON e.id = epg.environment_id").
		InnerJoin("project_members pm ON pm.project_id = e.project_id AND pm.role = epg.role").
		Where(builder.Eq{
			"pm.user_id": userID,
			"e.id":       environmentID,
		}).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	permissions := make(map[string]struct{})
	for _, grant := range grants {
		for _, key := range permissionKeysForPoint(grant.Permission) {
			permissions[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(permissions))
	for key := range permissions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func permissionKeysForPoint(point string) []string {
	switch point {
	case "READ":
		return []string{"project:read", "project:api_key:read", "config:read", "structure:read", "enum:read", "snapshot:read", "snapshot:export"}
	case "WRITE":
		return []string{"project:write", "config:write", "structure:write", "enum:write", "snapshot:write", "asset:write", "snapshot:import"}
	case "PUBLISH":
		return []string{"snapshot:publish"}
	case "API_KEY_MANAGE":
		return []string{"project:api_key:read", "project:api_key:manage"}
	default:
		return nil
	}
}

var _ PermissionRepository = (*PermissionRepositoryImpl)(nil)
