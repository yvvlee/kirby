package repository

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

var (
	ErrRoleInUse             = errors.New("role is assigned to environment members")
	ErrBuiltinRole           = errors.New("built-in role cannot be deleted")
	ErrSystemRolePermission  = errors.New("system permission cannot be assigned to an environment role")
	ErrPermissionSetMismatch = errors.New("permission set contains unknown permission")
)

type RoleUpdate struct {
	Name        string
	Description string
	UpdatedBy   int64
	Version     int64
}

type RoleWithPermissions struct {
	Role        model.Role
	Permissions []model.Permission
}

type RoleRepository interface {
	List(context.Context) ([]RoleWithPermissions, error)
	FindByID(context.Context, int64) (*RoleWithPermissions, error)
	Create(context.Context, *model.Role, *model.AuditLog) error
	Update(context.Context, int64, RoleUpdate, *model.AuditLog) error
	Delete(context.Context, int64, int64, *model.AuditLog) error
	UpdatePermissions(context.Context, int64, []int64, int64, *model.AuditLog) ([]PermissionAssignment, error)
}

type RoleRepositoryImpl struct {
	engine *xorm.Engine
	audits AuditLogRepository
}

func NewRoleRepository(engine *xorm.Engine, audits AuditLogRepository) *RoleRepositoryImpl {
	return &RoleRepositoryImpl{engine: engine, audits: audits}
}

func (r *RoleRepositoryImpl) List(ctx context.Context) ([]RoleWithPermissions, error) {
	roles := make([]model.Role, 0)
	if err := base.FindAll(ctx, r.engine, "roles", `
SELECT r.*
FROM roles AS r
WHERE r.deleted_at IS NULL
ORDER BY r.id ASC`, nil, &roles); err != nil {
		return nil, err
	}
	result := make([]RoleWithPermissions, 0, len(roles))
	for _, role := range roles {
		permissions, err := r.permissionsForRole(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, RoleWithPermissions{Role: role, Permissions: permissions})
	}
	return result, nil
}

func (r *RoleRepositoryImpl) FindByID(ctx context.Context, roleID int64) (*RoleWithPermissions, error) {
	if err := base.ValidateID("role_id", roleID); err != nil {
		return nil, err
	}
	var role model.Role
	if err := base.FindOne(ctx, r.engine, "role", `
SELECT r.*
FROM roles AS r
WHERE r.id = ? AND r.deleted_at IS NULL
LIMIT 1`, []any{roleID}, &role); err != nil {
		return nil, err
	}
	permissions, err := r.permissionsForRole(ctx, roleID)
	if err != nil {
		return nil, err
	}
	return &RoleWithPermissions{Role: role, Permissions: permissions}, nil
}

func (r *RoleRepositoryImpl) permissionsForRole(ctx context.Context, roleID int64) ([]model.Permission, error) {
	permissions := make([]model.Permission, 0)
	err := base.FindAll(ctx, r.engine, "role permissions", `
SELECT p.*
FROM role_permissions AS rp
INNER JOIN permissions AS p ON p.id = rp.permission_id
WHERE rp.role_id = ?
ORDER BY p.id ASC`, []any{roleID}, &permissions)
	return permissions, err
}

func (r *RoleRepositoryImpl) Create(ctx context.Context, role *model.Role, audit *model.AuditLog) error {
	if role == nil || audit == nil || r.audits == nil {
		return base.InvalidArgument("role and audit log are required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		result, err := base.ExecuteTx(ctx, tx, "role", `
INSERT INTO roles
    (`+"`key`"+`, name, description, builtin, created_by, updated_by)
VALUES (?, ?, ?, FALSE, ?, ?)`, role.Key, role.Name, role.Description, role.CreatedBy, role.UpdatedBy)
		if err != nil {
			return err
		}
		role.ID, err = result.LastInsertId()
		if err != nil {
			return base.Wrap("read inserted role id", err)
		}
		audit.ResourceID = strconv.FormatInt(role.ID, 10)
		return r.audits.RecordSystemTx(ctx, tx, audit)
	})
}

func (r *RoleRepositoryImpl) Update(ctx context.Context, roleID int64, update RoleUpdate, audit *model.AuditLog) error {
	if err := base.ValidateID("role_id", roleID); err != nil {
		return err
	}
	if update.Version < 0 || audit == nil || r.audits == nil {
		return base.InvalidArgument("valid version and audit log are required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		_, err := base.ExecuteTx(ctx, tx, "role", `
UPDATE roles
SET name = ?, description = ?, updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND version = ? AND deleted_at IS NULL`,
			update.Name, update.Description, update.UpdatedBy, roleID, update.Version)
		if err != nil {
			return err
		}
		audit.ResourceID = strconv.FormatInt(roleID, 10)
		return r.audits.RecordSystemTx(ctx, tx, audit)
	})
}

func (r *RoleRepositoryImpl) Delete(ctx context.Context, roleID, actorID int64, audit *model.AuditLog) error {
	if err := base.ValidateID("role_id", roleID); err != nil {
		return err
	}
	if audit == nil || r.audits == nil {
		return base.InvalidArgument("audit log is required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		var role model.Role
		if err := base.LockOne(ctx, tx, "role", `
SELECT r.*
FROM roles AS r
WHERE r.id = ? AND r.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{roleID}, &role); err != nil {
			return err
		}
		if role.Builtin {
			return ErrBuiltinRole
		}
		assignments := make([]PermissionAssignment, 0)
		if err := tx.Context(ctx).SQL(`
SELECT uer.user_id, uer.environment_id
FROM user_environment_roles AS uer
WHERE uer.role_id = ?
FOR UPDATE`, roleID).Find(&assignments); err != nil {
			return base.Wrap("lock role assignments", err)
		}
		if len(assignments) > 0 {
			return ErrRoleInUse
		}
		_, err := base.ExecuteTx(ctx, tx, "role", `
UPDATE roles
SET deleted_at = UTC_TIMESTAMP(6), updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND deleted_at IS NULL`, actorID, roleID)
		if err != nil {
			return err
		}
		audit.ResourceID = strconv.FormatInt(roleID, 10)
		return r.audits.RecordSystemTx(ctx, tx, audit)
	})
}

func (r *RoleRepositoryImpl) UpdatePermissions(ctx context.Context, roleID int64, permissionIDs []int64, actorID int64, audit *model.AuditLog) ([]PermissionAssignment, error) {
	if err := base.ValidateID("role_id", roleID); err != nil {
		return nil, err
	}
	if audit == nil || r.audits == nil {
		return nil, base.InvalidArgument("audit log is required")
	}
	hadPermissionIDs := len(permissionIDs) > 0
	permissionIDs = sortedUniquePositive(permissionIDs)
	if permissionIDs == nil && hadPermissionIDs {
		return nil, base.InvalidArgument("permission ids must be positive and unique")
	}
	assignments := make([]PermissionAssignment, 0)
	err := database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		var role model.Role
		if err := base.LockOne(ctx, tx, "role", `
SELECT r.*
FROM roles AS r
WHERE r.id = ? AND r.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{roleID}, &role); err != nil {
			return err
		}
		if len(permissionIDs) > 0 {
			permissions, err := lockPermissions(ctx, tx, permissionIDs)
			if err != nil {
				return err
			}
			if len(permissions) != len(permissionIDs) {
				return ErrPermissionSetMismatch
			}
			for _, item := range permissions {
				if strings.HasPrefix(item.Key, "system:") {
					return ErrSystemRolePermission
				}
			}
		}
		if err := tx.Context(ctx).SQL(`
SELECT DISTINCT uer.user_id, uer.environment_id
FROM user_environment_roles AS uer
INNER JOIN environments AS e ON e.id = uer.environment_id AND e.deleted_at IS NULL
WHERE uer.role_id = ?
ORDER BY uer.user_id ASC, uer.environment_id ASC
FOR UPDATE`, roleID).Find(&assignments); err != nil {
			return base.Wrap("lock role assignments", err)
		}
		if _, err := tx.Context(ctx).Exec("DELETE FROM role_permissions WHERE role_id = ?", roleID); err != nil {
			return base.Wrap("replace role permissions", err)
		}
		for _, permissionID := range permissionIDs {
			if _, err := tx.Context(ctx).Exec(
				"INSERT INTO role_permissions (role_id, permission_id, created_by) VALUES (?, ?, ?)",
				roleID, permissionID, actorID,
			); err != nil {
				return base.Wrap("insert role permission", err)
			}
		}
		if _, err := base.ExecuteTx(ctx, tx, "role", `
UPDATE roles
SET updated_by = ?, updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND deleted_at IS NULL`, actorID, roleID); err != nil {
			return err
		}
		environmentIDs := make([]int64, 0, len(assignments))
		for _, assignment := range assignments {
			environmentIDs = append(environmentIDs, assignment.EnvironmentID)
		}
		if err := bumpEnvironmentPermissionGenerations(ctx, tx, environmentIDs, actorID); err != nil {
			return err
		}
		audit.ResourceID = strconv.FormatInt(roleID, 10)
		return r.audits.RecordSystemTx(ctx, tx, audit)
	})
	return assignments, err
}

func lockPermissions(ctx context.Context, tx *xorm.Session, permissionIDs []int64) ([]model.Permission, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(permissionIDs)), ",")
	args := make([]any, len(permissionIDs))
	for index, id := range permissionIDs {
		args[index] = id
	}
	permissions := make([]model.Permission, 0, len(permissionIDs))
	query := "SELECT p.* FROM permissions AS p WHERE p.id IN (" + placeholders + ") ORDER BY p.id ASC FOR UPDATE"
	if err := tx.Context(ctx).SQL(query, args...).Find(&permissions); err != nil {
		return nil, base.Wrap("lock permissions", err)
	}
	return permissions, nil
}

func sortedUniquePositive(values []int64) []int64 {
	result := append([]int64(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	for index, value := range result {
		if value <= 0 || (index > 0 && result[index-1] == value) {
			return nil
		}
	}
	return result
}

var _ RoleRepository = (*RoleRepositoryImpl)(nil)
