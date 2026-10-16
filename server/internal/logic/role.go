package logic

import (
	"context"

	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type roleAuthorizer interface {
	RequireSystem(ctx context.Context, userID int64, required string) error
	Invalidate(ctx context.Context, userID, environmentID int64) error
}

type RoleLogic struct {
	roles        repository.RoleRepository
	permission   repository.PermissionRepository
	authorizer   roleAuthorizer
	transactions TransactionRunner
}

// NewRoleLogic constructs the RoleLogic with explicit application dependencies.
//
// Parameters: roles and permissions provide catalogs; authorizer enforces system permission and cleans caches; transactions commits role changes and audits together.
// Returns: the configured RoleLogic.
// Throws: none.
func NewRoleLogic(
	roles repository.RoleRepository,
	permissions repository.PermissionRepository,
	authorizer roleAuthorizer,
	transactions TransactionRunner,
) *RoleLogic {
	return &RoleLogic{
		roles:        roles,
		permission:   permissions,
		authorizer:   authorizer,
		transactions: transactions,
	}
}

// List returns the role catalog for an already authenticated requester.
//
// Parameters: ctx controls reads; actor is unused because the authenticated catalog is shared.
// Returns: live roles with permissions, or a database error.
// Throws: none; failures are returned as errors.
func (l *RoleLogic) List(ctx context.Context, _ permission.Actor) ([]repository.RoleWithPermissions, error) {
	return l.roles.List(ctx)
}

// Create requires system role-management permission and audits role creation.
//
// Parameters: ctx controls the operation; actor identifies the creator; role supplies the custom role fields.
// Returns: the persisted role with permissions, or an authorization or transaction error.
// Throws: none; failures are returned as errors.
func (l *RoleLogic) Create(ctx context.Context, actor permission.Actor, role *model.Role) (*repository.RoleWithPermissions, error) {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return nil, err
	}
	role.CreatedBy = actor.UserID
	role.UpdatedBy = actor.UserID
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.roles.Create(txctx, role, roleAudit(actor, "role.create", "role"))
	})
	if err != nil {
		return nil, err
	}
	return l.roles.FindByID(ctx, role.ID)
}

// Update requires system role-management permission before a version-guarded edit.
//
// Parameters: ctx controls the operation; actor identifies the editor; roleID selects the role; update supplies fields and expected version.
// Returns: the persisted role, or an authorization, version, or transaction error.
// Throws: none; failures are returned as errors.
func (l *RoleLogic) Update(ctx context.Context, actor permission.Actor, roleID int64, update repository.RoleUpdate) (*repository.RoleWithPermissions, error) {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return nil, err
	}
	update.UpdatedBy = actor.UserID
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.roles.Update(txctx, roleID, update, roleAudit(actor, "role.update", "role"))
	})
	if err != nil {
		return nil, err
	}
	return l.roles.FindByID(ctx, roleID)
}

// Delete requires system role-management permission before audited role deletion.
//
// Parameters: ctx controls the operation; actor identifies the editor; roleID selects the role.
// Returns: nil after commit, or an authorization, protected-role, membership, or transaction error.
// Throws: none; failures are returned as errors.
func (l *RoleLogic) Delete(ctx context.Context, actor permission.Actor, roleID int64) error {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return err
	}
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.roles.Delete(txctx, roleID, actor.UserID, roleAudit(actor, "role.delete", "role"))
	})
}

// ListPermissions limits the complete permission catalog to system role managers.
//
// Parameters: ctx controls reads; actor identifies the requester.
// Returns: permission catalog entries, or an authorization or database error.
// Throws: none; failures are returned as errors.
func (l *RoleLogic) ListPermissions(ctx context.Context, actor permission.Actor) ([]model.Permission, error) {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return nil, err
	}
	return l.permission.List(ctx)
}

// UpdatePermissions commits permission changes before attempting cleanup for every affected membership.
//
// Parameters: ctx controls the operation; actor identifies the editor; roleID selects the role; permissionIDs is the complete replacement set.
// Returns: nil after commit; authorization and transaction errors are returned, while all cache-cleanup failures are logged.
// Throws: none; failures are returned as errors.
func (l *RoleLogic) UpdatePermissions(ctx context.Context, actor permission.Actor, roleID int64, permissionIDs []int64) error {
	if err := l.authorizer.RequireSystem(ctx, actor.UserID, permission.SystemRoleManage); err != nil {
		return err
	}
	var assignments []repository.PermissionAssignment
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		assignments, err = l.roles.UpdatePermissions(txctx, roleID, permissionIDs, actor.UserID,
			roleAudit(actor, "role.permissions.update", "role"))
		return err
	})
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		// The transaction already advanced environments.version. Redis deletion
		// only reclaims stale keys and cannot affect the committed API result.
		if err := l.authorizer.Invalidate(ctx, assignment.UserID, assignment.EnvironmentID); err != nil {
			kratoslog.Context(ctx).Warnw("operation", "invalidate role permissions", "user_id", assignment.UserID, "environment_id", assignment.EnvironmentID, "error", err)
		}
	}
	return nil
}

func roleAudit(actor permission.Actor, action, resourceType string) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: resourceType,
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
