package repository

import (
	"context"
	"strconv"
	"strings"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type EnvironmentMemberRecord struct {
	User  model.User
	Roles []RoleWithPermissions
}

type UserEnvironmentRoleRepository interface {
	ListMembers(context.Context, int64) ([]EnvironmentMemberRecord, error)
	ReplaceRoles(context.Context, int64, int64, []int64, int64, *model.AuditLog) error
}

type UserEnvironmentRoleRepositoryImpl struct {
	engine *xorm.Engine
	audits AuditLogRepository
}

func NewUserEnvironmentRoleRepository(engine *xorm.Engine, audits AuditLogRepository) *UserEnvironmentRoleRepositoryImpl {
	return &UserEnvironmentRoleRepositoryImpl{engine: engine, audits: audits}
}

func (r *UserEnvironmentRoleRepositoryImpl) ListMembers(ctx context.Context, environmentID int64) ([]EnvironmentMemberRecord, error) {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return nil, err
	}
	users := make([]model.User, 0)
	if err := base.FindAll(ctx, r.engine, "environment members", `
SELECT DISTINCT u.*
FROM user_environment_roles AS uer
INNER JOIN users AS u ON u.id = uer.user_id AND u.deleted_at IS NULL
INNER JOIN environments AS e ON e.id = uer.environment_id AND e.deleted_at IS NULL
WHERE uer.environment_id = ?
ORDER BY u.id ASC`, []any{environmentID}, &users); err != nil {
		return nil, err
	}
	result := make([]EnvironmentMemberRecord, 0, len(users))
	for _, user := range users {
		roles, err := r.rolesForMember(ctx, environmentID, user.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, EnvironmentMemberRecord{User: user, Roles: roles})
	}
	return result, nil
}

func (r *UserEnvironmentRoleRepositoryImpl) rolesForMember(ctx context.Context, environmentID, userID int64) ([]RoleWithPermissions, error) {
	roles := make([]model.Role, 0)
	if err := base.FindAll(ctx, r.engine, "environment member roles", `
SELECT r.*
FROM user_environment_roles AS uer
INNER JOIN roles AS r ON r.id = uer.role_id AND r.deleted_at IS NULL
WHERE uer.environment_id = ? AND uer.user_id = ?
ORDER BY r.id ASC`, []any{environmentID, userID}, &roles); err != nil {
		return nil, err
	}
	result := make([]RoleWithPermissions, 0, len(roles))
	for _, role := range roles {
		permissions := make([]model.Permission, 0)
		if err := base.FindAll(ctx, r.engine, "environment member role permissions", `
SELECT p.*
FROM role_permissions AS rp
INNER JOIN permissions AS p ON p.id = rp.permission_id
WHERE rp.role_id = ?
ORDER BY p.id ASC`, []any{role.ID}, &permissions); err != nil {
			return nil, err
		}
		result = append(result, RoleWithPermissions{Role: role, Permissions: permissions})
	}
	return result, nil
}

func (r *UserEnvironmentRoleRepositoryImpl) ReplaceRoles(ctx context.Context, environmentID, userID int64, roleIDs []int64, actorID int64, audit *model.AuditLog) error {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return err
	}
	if err := base.ValidateID("user_id", userID); err != nil {
		return err
	}
	hadRoleIDs := len(roleIDs) > 0
	roleIDs = sortedUniquePositive(roleIDs)
	if roleIDs == nil && hadRoleIDs {
		return base.InvalidArgument("role ids must be positive and unique")
	}
	if audit == nil || r.audits == nil {
		return base.InvalidArgument("audit log is required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		var environment model.Environment
		if err := base.LockOne(ctx, tx, "environment", `
SELECT e.*
FROM environments AS e
WHERE e.id = ? AND e.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{environmentID}, &environment); err != nil {
			return err
		}
		var user model.User
		if err := base.LockOne(ctx, tx, "environment member", `
SELECT u.*
FROM users AS u
WHERE u.id = ? AND u.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{userID}, &user); err != nil {
			return err
		}
		if len(roleIDs) > 0 {
			roles, err := lockEnvironmentRoles(ctx, tx, roleIDs)
			if err != nil {
				return err
			}
			if len(roles) != len(roleIDs) {
				return base.Missing("environment role")
			}
			if err := ensureRolesContainNoSystemPermissions(ctx, tx, roleIDs); err != nil {
				return err
			}
		}
		if _, err := tx.Context(ctx).Exec(
			"DELETE FROM user_environment_roles WHERE user_id = ? AND environment_id = ?",
			userID, environmentID,
		); err != nil {
			return base.Wrap("replace environment member roles", err)
		}
		for _, roleID := range roleIDs {
			if _, err := tx.Context(ctx).Exec(
				"INSERT INTO user_environment_roles (user_id, environment_id, role_id, created_by) VALUES (?, ?, ?, ?)",
				userID, environmentID, roleID, actorID,
			); err != nil {
				return base.Wrap("insert environment member role", err)
			}
		}
		audit.ResourceID = strconv.FormatInt(userID, 10)
		return r.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit)
	})
}

func lockEnvironmentRoles(ctx context.Context, tx *xorm.Session, roleIDs []int64) ([]model.Role, error) {
	placeholders, args := idPlaceholders(roleIDs)
	roles := make([]model.Role, 0, len(roleIDs))
	query := "SELECT r.* FROM roles AS r WHERE r.id IN (" + placeholders + ") AND r.deleted_at IS NULL ORDER BY r.id ASC FOR UPDATE"
	if err := tx.Context(ctx).SQL(query, args...).Find(&roles); err != nil {
		return nil, base.Wrap("lock environment roles", err)
	}
	return roles, nil
}

func ensureRolesContainNoSystemPermissions(ctx context.Context, tx *xorm.Session, roleIDs []int64) error {
	placeholders, args := idPlaceholders(roleIDs)
	var row struct {
		Total int64 `xorm:"total"`
	}
	query := "SELECT COUNT(*) AS total FROM role_permissions AS rp " +
		"INNER JOIN permissions AS p ON p.id = rp.permission_id " +
		"WHERE rp.role_id IN (" + placeholders + ") AND p.`key` LIKE 'system:%'"
	found, err := tx.Context(ctx).SQL(query, args...).Get(&row)
	if err != nil {
		return base.Wrap("inspect environment role permissions", err)
	}
	if !found {
		return base.Missing("environment role permissions")
	}
	if row.Total > 0 {
		return ErrSystemRolePermission
	}
	return nil
}

func idPlaceholders(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

var _ UserEnvironmentRoleRepository = (*UserEnvironmentRoleRepositoryImpl)(nil)
