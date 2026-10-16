package repository

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type EnvironmentMemberRecord struct {
	User  model.User
	Roles []RoleWithPermissions
}

type MemberRoleUpdate struct {
	EnvironmentID int64
	UserID        int64
	RoleIDs       []int64
	ActorID       int64
}

type UserEnvironmentRoleRepository interface {
	ListMembers(ctx context.Context, environmentID int64) ([]EnvironmentMemberRecord, error)
	ReplaceRoles(ctx context.Context, update MemberRoleUpdate, audit *model.AuditLog) error
}

type UserEnvironmentRoleRepositoryImpl struct {
	*lorm.Repository[*model.UserEnvironmentRole]
	audits AuditLogRepository
}

// NewUserEnvironmentRoleRepository binds the UserEnvironmentRoleRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured UserEnvironmentRoleRepository.
// Throws: none.
func NewUserEnvironmentRoleRepository(storage *infra.Storage) *UserEnvironmentRoleRepositoryImpl {
	return &UserEnvironmentRoleRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.UserEnvironmentRole](),
		audits:     NewAuditLogRepository(storage),
	}
}

// ListMembers combines live environment users with their current roles and permissions.
//
// Parameters: ctx controls the reads; environmentID selects the membership scope.
// Returns: members ordered by user ID, or a database error.
// Throws: none; failures are returned as errors.
func (r *UserEnvironmentRoleRepositoryImpl) ListMembers(ctx context.Context, environmentID int64) ([]EnvironmentMemberRecord, error) {
	users, err := r.Engine.Query[*model.User]().From("users u").Select("u.*").
		InnerJoin("user_environment_roles uer ON uer.user_id = u.id").
		InnerJoin("environments e ON e.id = uer.environment_id AND e.deleted_at IS NULL").
		Where(builder.And{
			builder.Eq{
				"uer.environment_id": environmentID,
			},
			builder.IsNull("u.deleted_at"),
		}).Distinct().Asc("u.id").Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	result := make([]EnvironmentMemberRecord, 0, len(users))
	for _, user := range users {
		roles, err := r.rolesForMember(ctx, environmentID, user.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, EnvironmentMemberRecord{
			User:  *user,
			Roles: roles,
		})
	}
	return result, nil
}

func (r *UserEnvironmentRoleRepositoryImpl) rolesForMember(ctx context.Context, environmentID, userID int64) ([]RoleWithPermissions, error) {
	roles, err := r.Engine.Query[*model.Role]().From("roles r").Select("r.*").
		InnerJoin("user_environment_roles uer ON uer.role_id = r.id").
		Where(builder.And{
			builder.Eq{
				"uer.environment_id": environmentID,
				"uer.user_id":        userID,
			},
			builder.IsNull("r.deleted_at"),
		}).Asc("r.id").Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return rolesWithPermissions(ctx, r.Engine, roles)
}

// ReplaceRoles replaces one member assignment set and advances the environment generation atomically.
//
// Parameters: ctx carries the transaction; update supplies environment, user, role IDs, and actor; audit records the membership change.
// Returns: nil after assignments, generation, and audit succeed; missing resources, system-only roles, and write failures return errors.
// Throws: none; failures are returned as errors.
func (r *UserEnvironmentRoleRepositoryImpl) ReplaceRoles(ctx context.Context, update MemberRoleUpdate, audit *model.AuditLog) error {
	if err := r.lockMember(ctx, update.EnvironmentID, update.UserID); err != nil {
		return err
	}
	if err := r.lockRoles(ctx, update.RoleIDs); err != nil {
		return err
	}
	if err := r.replaceLinks(ctx, update.EnvironmentID, update.UserID, update.RoleIDs, update.ActorID); err != nil {
		return err
	}
	if err := bumpEnvironmentPermissionGenerations(ctx, r.Engine, []int64{update.EnvironmentID}, update.ActorID); err != nil {
		return err
	}
	audit.ResourceID = strconv.FormatInt(update.UserID, 10)
	return r.audits.RecordForEnvironment(ctx, update.EnvironmentID, audit)
}

func (r *UserEnvironmentRoleRepositoryImpl) lockMember(ctx context.Context, environmentID, userID int64) error {
	_, found, err := r.Engine.Query[*model.Environment]().ID(environmentID).
		Where(builder.IsNull(new(model.Environment).LormCols().DeletedAt())).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !found {
		return Missing("environment")
	}
	_, found, err = r.Engine.Query[*model.User]().ID(userID).
		Where(builder.IsNull(new(model.User).LormCols().DeletedAt())).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !found {
		return Missing("environment member")
	}
	return nil
}

func (r *UserEnvironmentRoleRepositoryImpl) lockRoles(ctx context.Context, roleIDs []int64) error {
	if len(roleIDs) == 0 {
		return nil
	}
	fields := new(model.Role).LormCols()
	roles, err := r.Engine.Query[*model.Role]().Where(builder.In(fields.ID(), roleIDs)).
		Where(builder.IsNull(fields.DeletedAt())).Asc(fields.ID()).Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if len(roles) != len(roleIDs) {
		return Missing("environment role")
	}
	return ensureRolesContainNoSystemPermissions(ctx, r.Engine, roleIDs)
}

func (r *UserEnvironmentRoleRepositoryImpl) replaceLinks(ctx context.Context, environmentID, userID int64, roleIDs []int64, actorID int64) error {
	fields := new(model.UserEnvironmentRole).LormCols()
	if _, err := r.Engine.Delete[*model.UserEnvironmentRole]().Where(builder.Eq{
		fields.UserID():        userID,
		fields.EnvironmentID(): environmentID,
	}).Exec(ctx); err != nil {
		return WrapDatabaseError(err)
	}
	if len(roleIDs) == 0 {
		return nil
	}
	links := make([]*model.UserEnvironmentRole, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		link := &model.UserEnvironmentRole{
			UserID:        userID,
			EnvironmentID: environmentID,
			RoleID:        roleID,
			CreatedBy:     actorID,
		}
		link.PrepareInsert()
		links = append(links, link)
	}
	_, err := r.InsertAll(ctx, links)
	return WrapDatabaseError(err)
}

func ensureRolesContainNoSystemPermissions(ctx context.Context, engine *lorm.Engine, roleIDs []int64) error {
	exists, err := engine.Query[*model.Permission]().From("permissions p").
		InnerJoin("role_permissions rp ON rp.permission_id = p.id").
		Where(builder.In("rp.role_id", roleIDs)).Where(builder.Like("p.key", "system:%")).Exist(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if exists {
		return ErrSystemRolePermission
	}
	return nil
}

// Permission generation updates follow a stable order to avoid environment lock inversions.
func bumpEnvironmentPermissionGenerations(ctx context.Context, engine *lorm.Engine, environmentIDs []int64, actorID int64) error {
	ordered := slices.Clone(environmentIDs)
	slices.Sort(ordered)
	for _, environmentID := range slices.Compact(ordered) {
		if err := bumpEnvironmentPermissionGeneration(ctx, engine, environmentID, actorID); err != nil {
			return err
		}
	}
	return nil
}

func bumpEnvironmentPermissionGeneration(ctx context.Context, engine *lorm.Engine, environmentID, actorID int64) error {
	fields := new(model.Environment).LormCols()
	affected, err := engine.Update[*model.Environment]().ID(environmentID).
		Where(builder.IsNull(fields.DeletedAt())).SetMap(map[string]any{
		fields.Version():   builder.Expr("version + 1"),
		fields.UpdatedBy(): actorID,
		fields.UpdatedAt(): time.Now().UTC(),
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("environment permission generation")
	}
	return nil
}

var _ UserEnvironmentRoleRepository = (*UserEnvironmentRoleRepositoryImpl)(nil)
