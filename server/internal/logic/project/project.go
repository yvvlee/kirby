package project

import (
	"context"
	"fmt"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type Repository interface {
	CreateTx(context.Context, *xorm.Session, int64, *model.Project) error
	FindByID(context.Context, int64, int64) (*model.Project, error)
	List(context.Context, int64, string, base.PageRequest) (base.PageResult[model.Project], error)
	UpdateTx(context.Context, *xorm.Session, int64, int64, repository.ProjectUpdate) error
}

type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}

type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

type Logic struct {
	projects     Repository
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
}

func New(projects Repository, permissions Authorizer, audits AuditRepository, transactions database.Transactor) (*Logic, error) {
	if projects == nil || permissions == nil || audits == nil || transactions == nil {
		return nil, fmt.Errorf("project logic dependencies are incomplete")
	}
	return &Logic{projects: projects, permissions: permissions, audits: audits, transactions: transactions}, nil
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, environmentID int64, item *model.Project) (*model.Project, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectWrite); err != nil {
		return nil, err
	}
	if item == nil {
		return nil, base.InvalidArgument("project is nil")
	}
	item.EnvironmentID = environmentID
	item.CreatedBy, item.UpdatedBy = actor.UserID, actor.UserID
	err := l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		if err := l.projects.CreateTx(ctx, tx, environmentID, item); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "project.create", "project", item.ID))
	})
	if err != nil {
		return nil, err
	}
	return l.projects.FindByID(ctx, environmentID, item.ID)
}

func (l *Logic) Update(ctx context.Context, actor permission.Actor, environmentID, projectID int64, update repository.ProjectUpdate) (*model.Project, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectWrite); err != nil {
		return nil, err
	}
	update.UpdatedBy = actor.UserID
	err := l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		if err := l.projects.UpdateTx(ctx, tx, environmentID, projectID, update); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "project.update", "project", projectID))
	})
	if err != nil {
		return nil, err
	}
	return l.projects.FindByID(ctx, environmentID, projectID)
}

func (l *Logic) List(ctx context.Context, actor permission.Actor, environmentID int64, keyword string) ([]model.Project, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectRead); err != nil {
		return nil, err
	}
	result := make([]model.Project, 0)
	for offset := 0; ; offset += base.MaxPageSize {
		page, err := l.projects.List(ctx, environmentID, keyword, base.PageRequest{Offset: offset, Limit: base.MaxPageSize})
		if err != nil {
			return nil, err
		}
		result = append(result, page.Items...)
		if len(result) >= int(page.Total) || len(page.Items) == 0 {
			return result, nil
		}
	}
}

func (l *Logic) Detail(ctx context.Context, actor permission.Actor, environmentID, projectID int64) (*model.Project, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectRead); err != nil {
		return nil, err
	}
	return l.projects.FindByID(ctx, environmentID, projectID)
}

func audit(actor permission.Actor, action, resourceType string, resourceID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{ActorUserID: &actorID, Action: action, ResourceType: resourceType,
		ResourceID: fmt.Sprintf("%d", resourceID), Result: model.AuditResultSucceeded, RequestID: actor.RequestID}
}
