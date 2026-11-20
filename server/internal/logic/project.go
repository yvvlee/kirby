package logic

import (
	"context"
	"fmt"
	"strings"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type projectAuthorizer interface {
	Require(ctx context.Context, userID, environmentID int64, permissions ...string) error
	RequireSystem(ctx context.Context, userID int64, permission string) error
}

type ProjectLogic struct {
	projects     repository.ProjectRepository
	members      repository.ProjectMemberRepository
	permissions  projectAuthorizer
	audits       repository.AuditLogRepository
	transactions TransactionRunner
}

// NewProjectLogicWithMembers constructs project operations with mandatory membership persistence.
//
// Parameters: projects, permissions, audits, transactions and members provide scoped persistence and authorization.
// Returns: the configured project operations.
// Throws: none.
func NewProjectLogicWithMembers(
	projects repository.ProjectRepository,
	permissions projectAuthorizer,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
	members repository.ProjectMemberRepository,
) *ProjectLogic {
	return &ProjectLogic{
		projects:     projects,
		members:      members,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
	}
}

// Create inserts a project and its system audit event in one transaction.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity and the requested project fields.
// Returns: the persisted project, or a permission, insertion, audit, reload, or conversion error.
// Throws: none.
func (l *ProjectLogic) Create(ctx context.Context, param *entity.CreateProjectParam) (*commonv1.Project, error) {
	actor := param.Actor
	item := &model.Project{
		Key:         param.Key,
		Name:        param.Name,
		Description: param.Description,
	}
	item.CreatedBy, item.UpdatedBy = actor.UserID, actor.UserID
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := l.projects.Create(txctx, 0, item); err != nil {
			return err
		}
		if err := l.members.Add(txctx, &model.ProjectMember{
			ProjectID: item.ID,
			UserID:    actor.UserID,
			Role:      string(permission.ProjectRoleOwner),
			CreatedBy: actor.UserID,
		}); err != nil {
			return err
		}
		_, err := l.audits.RecordSystem(txctx, projectAudit(actor, "project.create", "project", item.ID))
		return err
	})
	if err != nil {
		return nil, err
	}
	created, err := l.projects.FindByID(ctx, 0, item.ID)
	if err != nil {
		return nil, err
	}
	return converter.ProjectToProto(created)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, target project, and guarded changes.
// Returns: the updated project after commit, or a permission, stale-version, database, audit, or conversion error.
// Throws: none.
func (l *ProjectLogic) Update(ctx context.Context, param *entity.UpdateProjectParam) (*commonv1.Project, error) {
	actor := param.Actor
	update := param.Update
	update.UpdatedBy = actor.UserID
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := l.projects.LockByID(txctx, 0, param.ProjectID); err != nil {
			return err
		}
		if err := l.requireOwner(txctx, actor.UserID, param.ProjectID); err != nil {
			return err
		}
		if _, err := l.projects.Update(txctx, 0, param.ProjectID, update); err != nil {
			return err
		}
		_, err := l.audits.RecordSystem(txctx, projectAudit(actor, "project.update", "project", param.ProjectID))
		return err
	})
	if err != nil {
		return nil, err
	}
	updated, err := l.projects.FindByID(ctx, 0, param.ProjectID)
	if err != nil {
		return nil, err
	}
	return converter.ProjectToProto(updated)
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; param carries the caller identity, scope, and keyword.
// Returns: the authorized records, or a permission, repository, or conversion error.
// Throws: none.
func (l *ProjectLogic) List(ctx context.Context, param *entity.ListProjectParam) ([]*commonv1.Project, error) {
	actor := param.Actor
	if param.EnvironmentID > 0 {
		if err := l.permissions.Require(ctx, actor.UserID, param.EnvironmentID, permission.ProjectRead); err != nil {
			return nil, err
		}
	}
	if param.EnvironmentID == 0 {
		return l.listMemberProjects(ctx, actor.UserID, param.Keyword)
	}
	result := make([]model.Project, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.projects.List(ctx, param.EnvironmentID, param.Keyword, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, page.Items...)
		if len(result) >= int(page.Total) || len(page.Items) == 0 {
			return projectsToProto(result)
		}
	}
}

// Detail reads a project after checking the environment or system permission.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; projectID selects the project.
// Returns: the live project, or a permission, missing-project, database, or conversion error.
// Throws: none.
func (l *ProjectLogic) Detail(ctx context.Context, actor permission.Actor, environmentID, projectID int64) (*commonv1.Project, error) {
	member, err := l.members.IsMember(ctx, projectID, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, permission.ErrForbidden
	}

	item, err := l.projects.FindByID(ctx, environmentID, projectID)
	if err != nil {
		return nil, err
	}
	return converter.ProjectToProto(item)
}

func projectsToProto(items []model.Project) ([]*commonv1.Project, error) {
	result := make([]*commonv1.Project, 0, len(items))
	for index := range items {
		converted, err := converter.ProjectToProto(&items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

func projectAudit(actor permission.Actor, action, resourceType string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   fmt.Sprintf("%d", resourceID),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}

func (l *ProjectLogic) requireOwner(ctx context.Context, userID, projectID int64) error {
	member, err := l.members.Get(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if member.Role != string(permission.ProjectRoleOwner) {
		return permission.ErrForbidden
	}
	return nil
}

// Delete authorizes a current Owner under the project lock and removes its graph.
// Parameters: ctx controls the transaction; actor identifies the caller; projectID selects the project.
// Returns: nil or an authorization or database error.
// Throws: none; errors are returned.
func (l *ProjectLogic) Delete(ctx context.Context, actor permission.Actor, projectID int64) error {
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := l.projects.LockByID(txctx, 0, projectID); err != nil {
			return err
		}
		return l.deleteOwnedProject(txctx, actor, projectID)
	})
}
func (l *ProjectLogic) deleteOwnedProject(ctx context.Context, actor permission.Actor, projectID int64) error {
	member, err := l.members.Get(ctx, projectID, actor.UserID)
	if err != nil {
		return permission.ErrForbidden
	}
	if member.Role != string(permission.ProjectRoleOwner) {
		return permission.ErrForbidden
	}
	if err := l.projects.Delete(ctx, projectID); err != nil {
		return err
	}
	_, err = l.audits.RecordSystem(ctx, projectAudit(actor, "project.delete", "project", projectID))
	return err
}

func (l *ProjectLogic) listMemberProjects(ctx context.Context, userID int64, keyword string) ([]*commonv1.Project, error) {
	ids, err := l.members.ListProjectIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]model.Project, 0, len(ids))
	for _, projectID := range ids {
		item, err := l.projects.FindByID(ctx, 0, projectID)
		if err != nil {
			return nil, err
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), strings.ToLower(keyword)) && !strings.Contains(strings.ToLower(item.Description), strings.ToLower(keyword)) {
			continue
		}
		items = append(items, *item)
	}
	return projectsToProto(items)
}
