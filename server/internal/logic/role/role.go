package role

import (
	"context"
	"fmt"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type Repository interface {
	List(context.Context) ([]repository.RoleWithPermissions, error)
	FindByID(context.Context, int64) (*repository.RoleWithPermissions, error)
	Create(context.Context, *model.Role, *model.AuditLog) error
	Update(context.Context, int64, repository.RoleUpdate, *model.AuditLog) error
	Delete(context.Context, int64, int64, *model.AuditLog) error
	UpdatePermissions(context.Context, int64, []int64, int64, *model.AuditLog) ([]repository.PermissionAssignment, error)
}

type PermissionRepository interface {
	List(context.Context) ([]model.Permission, error)
}

type Authorizer interface {
	RequireSystem(context.Context, int64, string) error
	Invalidate(context.Context, int64, int64) error
}

type Logic struct {
	roles      Repository
	permission PermissionRepository
	authorizer Authorizer
}

func New(roles Repository, permissions PermissionRepository, authorizer Authorizer) (*Logic, error) {
	if roles == nil || permissions == nil || authorizer == nil {
		return nil, fmt.Errorf("role logic dependencies are incomplete")
	}
	return &Logic{roles: roles, permission: permissions, authorizer: authorizer}, nil
}

func (l *Logic) List(ctx context.Context, _ permission.Actor) ([]repository.RoleWithPermissions, error) {
	return l.roles.List(ctx)
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, role *model.Role) (*repository.RoleWithPermissions, error) {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return nil, err
	}
	if role == nil {
		return nil, fmt.Errorf("role is nil")
	}
	role.CreatedBy = actor.UserID
	role.UpdatedBy = actor.UserID
	role.Builtin = false
	if err := l.roles.Create(ctx, role, audit(actor, "role.create", "role")); err != nil {
		return nil, err
	}
	return l.roles.FindByID(ctx, role.ID)
}

func (l *Logic) Update(ctx context.Context, actor permission.Actor, roleID int64, update repository.RoleUpdate) (*repository.RoleWithPermissions, error) {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return nil, err
	}
	update.UpdatedBy = actor.UserID
	if err := l.roles.Update(ctx, roleID, update, audit(actor, "role.update", "role")); err != nil {
		return nil, err
	}
	return l.roles.FindByID(ctx, roleID)
}

func (l *Logic) Delete(ctx context.Context, actor permission.Actor, roleID int64) error {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return err
	}
	return l.roles.Delete(ctx, roleID, actor.UserID, audit(actor, "role.delete", "role"))
}

func (l *Logic) ListPermissions(ctx context.Context, actor permission.Actor) ([]model.Permission, error) {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return nil, err
	}
	return l.permission.List(ctx)
}

func (l *Logic) UpdatePermissions(ctx context.Context, actor permission.Actor, roleID int64, permissionIDs []int64) error {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return err
	}
	assignments, err := l.roles.UpdatePermissions(ctx, roleID, permissionIDs, actor.UserID,
		audit(actor, "role.permissions.update", "role"))
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		// The transaction already advanced environments.version. Redis deletion
		// only reclaims stale keys and cannot affect the committed API result.
		_ = l.authorizer.Invalidate(ctx, assignment.UserID, assignment.EnvironmentID)
	}
	return nil
}

func audit(actor permission.Actor, action, resourceType string) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID: &actorID, Action: action, ResourceType: resourceType,
		Result: model.AuditResultSucceeded, RequestID: actor.RequestID,
	}
}
