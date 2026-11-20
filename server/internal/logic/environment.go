package logic

import (
	"context"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
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
	projectMembers repository.ProjectMemberRepository
	environments   repository.EnvironmentRepository
	users          repository.UserRepository
	permissions    environmentAuthorizer
	transactions   TransactionRunner
}

// NewEnvironmentLogic constructs the EnvironmentLogic with explicit application dependencies.
//
// Parameters: environments, users, and members provide persistence; permissions authorizes operations; transactions commits writes and audits together.
// Returns: the configured EnvironmentLogic.
// Throws: none.
func NewEnvironmentLogic(
	environments repository.EnvironmentRepository,
	users repository.UserRepository,
	permissions environmentAuthorizer,
	transactions TransactionRunner,
	projectMembers repository.ProjectMemberRepository,
) *EnvironmentLogic {
	return &EnvironmentLogic{
		projectMembers: projectMembers,
		environments:   environments,
		users:          users,
		permissions:    permissions,
		transactions:   transactions,
	}
}

// List returns only environments belonging to projects the requester has joined.
//
// Parameters: ctx controls reads; actor identifies the requester.
// Returns: visible environments, or an authorization, database, or conversion error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) List(ctx context.Context, actor permission.Actor) ([]*commonv1.Environment, error) {
	user, err := l.users.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	items, err := l.users.ListEnvironments(ctx, user)
	if err != nil {
		return nil, err
	}
	return environmentsToProto(items)
}

// Create authorizes a project Owner before creating an environment with an audit.
//
// Parameters: ctx controls the operation; param carries the caller identity and the requested environment fields.
// Returns: the persisted environment, or an authorization, transaction, or conversion error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) Create(ctx context.Context, param *entity.CreateEnvironmentParam) (*commonv1.Environment, error) {
	actor := param.Actor
	if err := l.requireProjectOwner(ctx, actor.UserID, param.ProjectID); err != nil {
		return nil, err
	}
	environment := &model.Environment{
		ProjectID:   param.ProjectID,
		Key:         param.Key,
		Name:        param.Name,
		Description: param.Description,
	}
	environment.CreatedBy = actor.UserID
	environment.UpdatedBy = actor.UserID
	environment.Enabled = true
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		_, err := l.environments.Create(txctx, environment, environmentAudit(actor, "environment.create", "environment"))
		return err
	}); err != nil {
		return nil, err
	}
	created, err := l.environments.FindByID(ctx, environment.ID)
	if err != nil {
		return nil, err
	}
	return converter.EnvironmentToProto(created)
}

// Update authorizes a project Owner and records a version-guarded environment edit.
//
// Parameters: ctx controls the operation; param carries the caller identity, target environment, and guarded changes.
// Returns: the persisted environment, or an authorization, version, transaction, or conversion error.
// Throws: none; failures are returned as errors.
func (l *EnvironmentLogic) Update(ctx context.Context, param *entity.UpdateEnvironmentParam) (*commonv1.Environment, error) {
	actor := param.Actor
	current, err := l.environments.FindByID(ctx, param.EnvironmentID)
	if err != nil {
		return nil, err
	}
	if err := l.requireProjectOwner(ctx, actor.UserID, current.ProjectID); err != nil {
		return nil, err
	}
	update := param.Update
	update.UpdatedBy = actor.UserID
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		_, err := l.environments.Update(txctx, param.EnvironmentID, update, environmentAudit(actor, "environment.update", "environment"))
		return err
	}); err != nil {
		return nil, err
	}
	updated, err := l.environments.FindByID(ctx, param.EnvironmentID)
	if err != nil {
		return nil, err
	}
	return converter.EnvironmentToProto(updated)
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

func environmentsToProto(items []model.Environment) ([]*commonv1.Environment, error) {
	result := make([]*commonv1.Environment, 0, len(items))
	for index := range items {
		converted, err := converter.EnvironmentToProto(&items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
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

func (l *EnvironmentLogic) requireProjectOwner(ctx context.Context, userID, projectID int64) error {
	member, err := l.projectMembers.Get(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if member.Role != string(permission.ProjectRoleOwner) {
		return permission.ErrForbidden
	}
	return nil
}
