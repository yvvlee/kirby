package logic

import (
	"context"
	"errors"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type EnvironmentPermissionLogic struct {
	grants       repository.EnvironmentPermissionRepository
	environments repository.EnvironmentRepository
	members      repository.ProjectMemberRepository
	transactions TransactionRunner
}

// NewEnvironmentPermissionLogic enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func NewEnvironmentPermissionLogic(grants repository.EnvironmentPermissionRepository, environments repository.EnvironmentRepository, members repository.ProjectMemberRepository, transactions TransactionRunner) *EnvironmentPermissionLogic {
	return &EnvironmentPermissionLogic{
		grants:       grants,
		environments: environments,
		members:      members,
		transactions: transactions,
	}
}

// Matrix enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func (l *EnvironmentPermissionLogic) Matrix(ctx context.Context, actor permission.Actor, environmentID int64) ([]*adminv1.EnvironmentPermissionRole, error) {
	if err := l.requireManager(ctx, actor.UserID, environmentID); err != nil {
		return nil, err
	}
	items, err := l.grants.List(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	return matrixToProto(items), nil
}

// Update enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func (l *EnvironmentPermissionLogic) Update(ctx context.Context, actor permission.Actor, environmentID int64, roles []*adminv1.EnvironmentPermissionRole) ([]*adminv1.EnvironmentPermissionRole, error) {
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		if err := l.grants.LockScope(txctx, environmentID); err != nil {
			return err
		}
		if err := l.requireManager(txctx, actor.UserID, environmentID); err != nil {
			return err
		}
		return l.replaceRoles(txctx, actor.UserID, environmentID, roles)
	})
	if err != nil {
		return nil, err
	}
	return l.Matrix(ctx, actor, environmentID)
}

func (l *EnvironmentPermissionLogic) replaceRoles(ctx context.Context, actorID, environmentID int64, roles []*adminv1.EnvironmentPermissionRole) error {
	for _, role := range roles {
		if err := l.grants.Replace(ctx, environmentID, role.Role, role.Permissions, actorID); err != nil {
			return err
		}
	}
	return nil
}

func (l *EnvironmentPermissionLogic) requireManager(ctx context.Context, userID, environmentID int64) error {
	environment, err := l.environments.FindByID(ctx, environmentID)
	if err != nil {
		return err
	}
	member, err := l.members.Get(ctx, environment.ProjectID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return permission.ErrForbidden
		}
		return err
	}
	if member.Role != string(permission.ProjectRoleOwner) && member.Role != string(permission.ProjectRoleMaintainer) {
		return permission.ErrForbidden
	}
	return nil
}

func matrixToProto(items []model.EnvironmentPermissionGrant) []*adminv1.EnvironmentPermissionRole {
	result := make([]*adminv1.EnvironmentPermissionRole, 0, 4)
	byRole := map[string]*adminv1.EnvironmentPermissionRole{}
	for _, key := range []string{"OWNER", "MAINTAINER", "MEMBER", "GUEST"} {
		role := &adminv1.EnvironmentPermissionRole{
			Role:        key,
			Permissions: []string{},
		}
		byRole[key] = role
		result = append(result, role)
	}
	for _, item := range items {
		byRole[item.Role].Permissions = append(byRole[item.Role].Permissions, item.Permission)
	}
	byRole["OWNER"].Permissions = []string{"READ", "WRITE", "PUBLISH", "API_KEY_MANAGE"}
	return result
}
