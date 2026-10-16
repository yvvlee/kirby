package logic

import (
	"context"
	"fmt"

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
	permissions  projectAuthorizer
	audits       repository.AuditLogRepository
	transactions TransactionRunner
}

// NewProjectLogic creates ProjectLogic with its configuration dependencies.
//
// Parameters: projects checks project ownership; permissions checks the caller permissions; audits records successful changes in the transaction; transactions commits state changes and audit events together.
// Returns: *ProjectLogic.
// Throws: none.
func NewProjectLogic(
	projects repository.ProjectRepository,
	permissions projectAuthorizer,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
) *ProjectLogic {
	return &ProjectLogic{
		projects:     projects,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
	}
}

// Create inserts a project and its system audit event in one transaction.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; item contains the persisted record.
// Returns: the persisted project, or a permission, insertion, audit, or reload error.
// Throws: none.
func (l *ProjectLogic) Create(ctx context.Context, actor permission.Actor, environmentID int64, item *model.Project) (*model.Project, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemProjectManage); err != nil {
		return nil, err
	}
	item.EnvironmentID = environmentID
	item.CreatedBy, item.UpdatedBy = actor.UserID, actor.UserID
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		if err := l.projects.Create(txctx, environmentID, item); err != nil {
			return err
		}
		return l.audits.RecordSystem(txctx, projectAudit(actor, "project.create", "project", item.ID))
	})
	if err != nil {
		return nil, err
	}
	return l.projects.FindByID(ctx, 0, item.ID)
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; projectID selects the project; update contains the guarded changes and expected version.
// Returns: the updated project after commit, or a permission, stale-version, database, or audit error.
// Throws: none.
func (l *ProjectLogic) Update(ctx context.Context, actor permission.Actor, environmentID, projectID int64, update repository.ProjectUpdate) (*model.Project, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemProjectManage); err != nil {
		return nil, err
	}
	update.UpdatedBy = actor.UserID
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		if err := l.projects.Update(txctx, environmentID, projectID, update); err != nil {
			return err
		}
		return l.audits.RecordSystem(txctx, projectAudit(actor, "project.update", "project", projectID))
	})
	if err != nil {
		return nil, err
	}
	return l.projects.FindByID(ctx, 0, projectID)
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; keyword matches project names or descriptions.
// Returns: the authorized records, or a permission or repository error.
// Throws: none.
func (l *ProjectLogic) List(ctx context.Context, actor permission.Actor, environmentID int64, keyword string) ([]model.Project, error) {
	if environmentID > 0 {
		if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectRead); err != nil {
			return nil, err
		}
	} else if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemProjectManage); err != nil {
		return nil, err
	}
	result := make([]model.Project, 0)
	for offset := 0; ; offset += repository.MaxPageSize {
		page, err := l.projects.List(ctx, environmentID, keyword, repository.PageRequest{
			Offset: offset,
			Limit:  repository.MaxPageSize,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, page.Items...)
		if len(result) >= int(page.Total) || len(page.Items) == 0 {
			return result, nil
		}
	}
}

// Detail reads a project after checking the environment or system permission.
//
// Parameters: ctx carries cancellation and transaction state; actor identifies the authenticated caller; environmentID selects the authorized environment; projectID selects the project.
// Returns: the live project, or a permission, missing-project, or database error.
// Throws: none.
func (l *ProjectLogic) Detail(ctx context.Context, actor permission.Actor, environmentID, projectID int64) (*model.Project, error) {
	if environmentID > 0 {
		if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectRead); err != nil {
			return nil, err
		}
	} else if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemProjectManage); err != nil {
		return nil, err
	}
	return l.projects.FindByID(ctx, environmentID, projectID)
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
