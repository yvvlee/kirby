package repository

import (
	"context"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type PermissionIdentity struct {
	SystemAdmin        bool  `xorm:"is_system_admin"`
	EnvironmentID      int64 `xorm:"environment_id"`
	EnvironmentEnabled bool  `xorm:"environment_enabled"`
	EnvironmentMember  bool  `xorm:"environment_member"`
}

type PermissionAssignment struct {
	UserID        int64 `xorm:"user_id"`
	EnvironmentID int64 `xorm:"environment_id"`
}

type PermissionRepository interface {
	Identity(context.Context, int64, int64) (PermissionIdentity, error)
	SystemAdmin(context.Context, int64) (bool, error)
	KeysForUserEnvironment(context.Context, int64, int64) ([]string, error)
	List(context.Context) ([]model.Permission, error)
	AssignmentsForRole(context.Context, int64) ([]PermissionAssignment, error)
}

type PermissionRepositoryImpl struct{ engine *xorm.Engine }

func NewPermissionRepository(engine *xorm.Engine) *PermissionRepositoryImpl {
	return &PermissionRepositoryImpl{engine: engine}
}

func (r *PermissionRepositoryImpl) Identity(ctx context.Context, userID, environmentID int64) (PermissionIdentity, error) {
	if err := base.ValidateID("user_id", userID); err != nil {
		return PermissionIdentity{}, err
	}
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return PermissionIdentity{}, err
	}
	var identity PermissionIdentity
	err := base.FindOne(ctx, r.engine, "permission identity", `
SELECT u.is_system_admin,
       COALESCE(e.id, 0) AS environment_id,
       COALESCE(e.enabled, FALSE) AS environment_enabled,
       EXISTS(
           SELECT 1
           FROM user_environment_roles AS uer
           WHERE uer.user_id = u.id AND uer.environment_id = e.id
       ) AS environment_member
FROM users AS u
LEFT JOIN environments AS e
  ON e.id = ? AND e.deleted_at IS NULL
WHERE u.id = ? AND u.enabled = TRUE AND u.deleted_at IS NULL
LIMIT 1`, []any{environmentID, userID}, &identity)
	return identity, err
}

func (r *PermissionRepositoryImpl) SystemAdmin(ctx context.Context, userID int64) (bool, error) {
	if err := base.ValidateID("user_id", userID); err != nil {
		return false, err
	}
	var row struct {
		SystemAdmin bool `xorm:"is_system_admin"`
	}
	err := base.FindOne(ctx, r.engine, "system administrator", `
SELECT u.is_system_admin
FROM users AS u
WHERE u.id = ? AND u.enabled = TRUE AND u.deleted_at IS NULL
LIMIT 1`, []any{userID}, &row)
	return row.SystemAdmin, err
}

func (r *PermissionRepositoryImpl) KeysForUserEnvironment(ctx context.Context, userID, environmentID int64) ([]string, error) {
	if err := base.ValidateID("user_id", userID); err != nil {
		return nil, err
	}
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return nil, err
	}
	rows := make([]struct {
		Key string `xorm:"key"`
	}, 0)
	err := base.FindAll(ctx, r.engine, "environment permissions", `
SELECT DISTINCT p.`+"`key`"+`
FROM user_environment_roles AS uer
INNER JOIN roles AS r ON r.id = uer.role_id AND r.deleted_at IS NULL
INNER JOIN role_permissions AS rp ON rp.role_id = r.id
INNER JOIN permissions AS p ON p.id = rp.permission_id
WHERE uer.user_id = ? AND uer.environment_id = ?
ORDER BY p.`+"`key`"+` ASC`, []any{userID, environmentID}, &rows)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys, nil
}

func (r *PermissionRepositoryImpl) List(ctx context.Context) ([]model.Permission, error) {
	permissions := make([]model.Permission, 0)
	err := base.FindAll(ctx, r.engine, "permissions", `
SELECT p.*
FROM permissions AS p
ORDER BY p.id ASC`, nil, &permissions)
	return permissions, err
}

func (r *PermissionRepositoryImpl) AssignmentsForRole(ctx context.Context, roleID int64) ([]PermissionAssignment, error) {
	if err := base.ValidateID("role_id", roleID); err != nil {
		return nil, err
	}
	assignments := make([]PermissionAssignment, 0)
	err := base.FindAll(ctx, r.engine, "role assignments", `
SELECT DISTINCT uer.user_id, uer.environment_id
FROM user_environment_roles AS uer
WHERE uer.role_id = ?
ORDER BY uer.user_id ASC, uer.environment_id ASC`, []any{roleID}, &assignments)
	return assignments, err
}

var _ PermissionRepository = (*PermissionRepositoryImpl)(nil)
