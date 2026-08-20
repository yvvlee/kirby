package environment

import (
	"context"
	"errors"
	"fmt"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type EnvironmentRepository interface {
	List(context.Context) ([]model.Environment, error)
	FindByID(context.Context, int64) (*model.Environment, error)
	Create(context.Context, *model.Environment, *model.AuditLog) error
	Update(context.Context, int64, repository.EnvironmentUpdate, *model.AuditLog) error
}

type UserRepository interface {
	GetByID(context.Context, int64) (*model.User, error)
	ListEnvironments(context.Context, *model.User) ([]model.Environment, error)
}

type MemberRepository interface {
	ListMembers(context.Context, int64) ([]repository.EnvironmentMemberRecord, error)
	ReplaceRoles(context.Context, int64, int64, []int64, int64, *model.AuditLog) error
}

type Authorizer interface {
	Resolve(context.Context, int64, int64) ([]string, bool, error)
	Require(context.Context, int64, int64, ...string) error
	RequireSystem(context.Context, int64, string) error
	Invalidate(context.Context, int64, int64) error
}

type Logic struct {
	environments EnvironmentRepository
	users        UserRepository
	members      MemberRepository
	permissions  Authorizer
}

func New(environments EnvironmentRepository, users UserRepository, members MemberRepository, permissions Authorizer) (*Logic, error) {
	if environments == nil || users == nil || members == nil || permissions == nil {
		return nil, fmt.Errorf("environment logic dependencies are incomplete")
	}
	return &Logic{environments: environments, users: users, members: members, permissions: permissions}, nil
}

func (l *Logic) List(ctx context.Context, actor permission.Actor) ([]model.Environment, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemEnvironmentManage); err == nil {
		return l.environments.List(ctx)
	} else if !errors.Is(err, permission.ErrForbidden) {
		return nil, err
	}
	user, err := l.users.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	return l.users.ListEnvironments(ctx, user)
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, environment *model.Environment) (*model.Environment, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemEnvironmentManage); err != nil {
		return nil, err
	}
	if environment == nil {
		return nil, fmt.Errorf("environment is nil")
	}
	environment.CreatedBy = actor.UserID
	environment.UpdatedBy = actor.UserID
	environment.Enabled = true
	if err := l.environments.Create(ctx, environment, audit(actor, "environment.create", "environment")); err != nil {
		return nil, err
	}
	return l.environments.FindByID(ctx, environment.ID)
}

func (l *Logic) Update(ctx context.Context, actor permission.Actor, environmentID int64, update repository.EnvironmentUpdate) (*model.Environment, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemEnvironmentManage); err != nil {
		return nil, err
	}
	update.UpdatedBy = actor.UserID
	if err := l.environments.Update(ctx, environmentID, update, audit(actor, "environment.update", "environment")); err != nil {
		return nil, err
	}
	return l.environments.FindByID(ctx, environmentID)
}

// VerifyProject checks the project path parameter before a scoped environment
// update. The HTTP route contains both IDs, so accepting a mismatched pair
// would allow an environment to be edited through another project's URL.
func (l *Logic) VerifyProject(ctx context.Context, environmentID, projectID int64) error {
	item, err := l.environments.FindByID(ctx, environmentID)
	if err != nil {
		return err
	}
	if item.ProjectID != projectID {
		return fmt.Errorf("environment does not belong to project")
	}
	return nil
}

func (l *Logic) Permissions(ctx context.Context, actor permission.Actor, environmentID int64) ([]string, error) {
	keys, _, err := l.permissions.Resolve(ctx, actor.UserID, environmentID)
	return keys, err
}

func (l *Logic) ListMembers(ctx context.Context, actor permission.Actor, environmentID int64) ([]repository.EnvironmentMemberRecord, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnvironmentMemberManage); err != nil {
		return nil, err
	}
	return l.members.ListMembers(ctx, environmentID)
}

func (l *Logic) UpdateMemberRoles(ctx context.Context, actor permission.Actor, environmentID, userID int64, roleIDs []int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.EnvironmentMemberManage); err != nil {
		return err
	}
	if err := l.members.ReplaceRoles(ctx, environmentID, userID, roleIDs, actor.UserID,
		audit(actor, "environment.member.roles.update", "user")); err != nil {
		return err
	}
	// The transaction already advanced environments.version. Redis deletion is
	// only stale-key cleanup and must not turn a committed change into an API
	// failure.
	_ = l.permissions.Invalidate(ctx, userID, environmentID)
	return nil
}

func audit(actor permission.Actor, action, resourceType string) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID: &actorID, Action: action, ResourceType: resourceType,
		Result: model.AuditResultSucceeded, RequestID: actor.RequestID,
	}
}
