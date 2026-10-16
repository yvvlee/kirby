package logic

import (
	"context"

	"errors"

	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type environmentAuthorizer interface {
	Resolve(ctx context.Context, userID, environmentID int64) ([]string, bool, error)
	Require(ctx context.Context, userID, environmentID int64, required ...string) error
	RequireSystem(ctx context.Context, userID int64, required string) error
	Invalidate(ctx context.Context, userID, environmentID int64) error
}

type EnvironmentLogic struct {
	environments repository.EnvironmentRepository
	users        repository.UserRepository
	members      repository.UserEnvironmentRoleRepository
	permissions  environmentAuthorizer
	transactions TransactionRunner
}

// NewEnvironmentLogic constructs the EnvironmentLogic with explicit application dependencies.
//
// Parameters: environments, users, and members provide persistence; permissions authorizes operations; transactions commits writes and audits together.
// Returns: the configured EnvironmentLogic.
// Throws: none.
func NewEnvironmentLogic(
	environments repository.EnvironmentRepository,
	users repository.UserRepository,
	members repository.UserEnvironmentRoleRepository,
	permissions environmentAuthorizer,
	transactions TransactionRunner,
) *EnvironmentLogic {
	return &EnvironmentLogic{
		environments: environments,
		users:        users,
		members:      members,
		permissions:  permissions,
		transactions: transactions,
	}
}

// List gives system managers all environments and other accounts only their visible environments.
//
// Parameters: ctx controls reads; actor identifies the requester.
// Returns: visible environments, or an authorization or database error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) List(ctx context.Context, actor permission.Actor) ([]model.Environment, error) {
	permissionErr := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemEnvironmentManage)
	if permissionErr == nil {
		return l.environments.List(ctx)
	}
	if !errors.Is(permissionErr, permission.ErrForbidden) {
		return nil, permissionErr
	}
	user, err := l.users.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	return l.users.ListEnvironments(ctx, user)
}

// Create authorizes a system manager before creating an environment with an audit.
//
// Parameters: ctx controls the operation; actor identifies the creator; environment contains project ownership and requested fields.
// Returns: the persisted environment, or an authorization or transaction error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) Create(ctx context.Context, actor permission.Actor, environment *model.Environment) (*model.Environment, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemEnvironmentManage); err != nil {
		return nil, err
	}
	environment.CreatedBy = actor.UserID
	environment.UpdatedBy = actor.UserID
	environment.Enabled = true
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.environments.Create(txctx, environment, environmentAudit(actor, "environment.create", "environment"))
	}); err != nil {
		return nil, err
	}
	return l.environments.FindByID(ctx, environment.ID)
}

// Update authorizes a system manager and records a version-guarded environment edit.
//
// Parameters: ctx controls the operation; actor identifies the editor; environmentID selects the verified scope; update supplies fields and expected version.
// Returns: the persisted environment, or an authorization, version, or transaction error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) Update(ctx context.Context, actor permission.Actor, environmentID int64, update repository.EnvironmentUpdate) (*model.Environment, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemEnvironmentManage); err != nil {
		return nil, err
	}
	update.UpdatedBy = actor.UserID
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.environments.Update(txctx, environmentID, update, environmentAudit(actor, "environment.update", "environment"))
	}); err != nil {
		return nil, err
	}
	return l.environments.FindByID(ctx, environmentID)
}

// VerifyProject rejects a route whose project does not own the requested environment.
//
// Parameters: ctx controls the read; environmentID selects the environment; projectID is the route owner.
// Returns: nil for matching ownership, ErrNotFound for mismatches, or a database error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) VerifyProject(ctx context.Context, environmentID, projectID int64) error {
	item, err := l.environments.FindByID(ctx, environmentID)
	if err != nil {
		return err
	}
	if item.ProjectID != projectID {
		return repository.Missing("environment")
	}
	return nil
}

// Permissions resolves permissions against the current environment generation.
//
// Parameters: ctx controls resolution; actor identifies the requester; environmentID defines the authorization scope.
// Returns: current permission keys, or an authorization or dependency error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) Permissions(ctx context.Context, actor permission.Actor, environmentID int64) ([]string, error) {
	keys, _, err := l.permissions.Resolve(ctx, actor.UserID, environmentID)
	return keys, err
}

// ListMembers requires member-management permission before exposing environment membership.
//
// Parameters: ctx controls reads; actor identifies the requester; environmentID defines the scope.
// Returns: member records, or an authorization or database error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) ListMembers(ctx context.Context, actor permission.Actor, environmentID int64) ([]repository.EnvironmentMemberRecord, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnvironmentMemberManage); err != nil {
		return nil, err
	}
	return l.members.ListMembers(ctx, environmentID)
}

// UpdateMemberRoles commits role assignment changes before observable cleanup of the previous cache generation.
//
// Parameters: ctx controls the operation; actor identifies the editor; environmentID and userID identify membership; roleIDs is the complete replacement set.
// Returns: nil after the transaction commits; authorization and transaction errors are returned, while cache-cleanup errors are logged.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) UpdateMemberRoles(ctx context.Context, actor permission.Actor, environmentID, userID int64, roleIDs []int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnvironmentMemberManage); err != nil {
		return err
	}
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.members.ReplaceRoles(txctx, repository.MemberRoleUpdate{
			EnvironmentID: environmentID,
			UserID:        userID,
			RoleIDs:       roleIDs,
			ActorID:       actor.UserID,
		},
			environmentAudit(actor, "environment.member.roles.update", "user"))
	}); err != nil {
		return err
	}
	// The transaction already advanced environments.version. Redis deletion is
	// only stale-key cleanup and must not turn a committed change into an API
	// failure.
	if err := l.permissions.Invalidate(ctx, userID, environmentID); err != nil {
		kratoslog.Context(ctx).Warnw("operation", "invalidate member permissions", "user_id", userID, "environment_id", environmentID, "error", err)
	}
	return nil
}

func environmentAudit(actor permission.Actor, action, resourceType string) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: resourceType,
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
