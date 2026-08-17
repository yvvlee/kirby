package repository

import (
	"context"
	"strings"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type ProjectUpdate struct {
	Name        string
	Description string
	UpdatedBy   int64
	Version     int64
}

type ProjectRepository interface {
	Create(context.Context, int64, *model.Project) error
	CreateTx(context.Context, *xorm.Session, int64, *model.Project) error
	FindByID(context.Context, int64, int64) (*model.Project, error)
	FindByKey(context.Context, int64, string) (*model.Project, error)
	List(context.Context, int64, string, base.PageRequest) (base.PageResult[model.Project], error)
	Update(context.Context, int64, int64, ProjectUpdate) error
	UpdateTx(context.Context, *xorm.Session, int64, int64, ProjectUpdate) error
	LockByID(context.Context, *xorm.Session, int64, int64) (*model.Project, error)
}

func (r *ProjectRepositoryImpl) CreateTx(ctx context.Context, tx *xorm.Session, environmentID int64, project *model.Project) error {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return err
	}
	if project == nil {
		return base.InvalidArgument("project is nil")
	}
	if project.EnvironmentID != 0 && project.EnvironmentID != environmentID {
		return base.InvalidArgument("project.environment_id does not match environment_id")
	}
	project.EnvironmentID = environmentID
	result, err := base.ExecuteTx(ctx, tx, "project", `
INSERT INTO projects
    (environment_id, `+"`key`"+`, name, description, created_by, updated_by)
SELECT e.id, ?, ?, ?, ?, ?
FROM environments AS e
WHERE e.id = ? AND e.deleted_at IS NULL`,
		project.Key, project.Name, project.Description, project.CreatedBy, project.UpdatedBy, environmentID)
	if err != nil {
		return err
	}
	project.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted project id", err)
	}
	return nil
}

type ProjectRepositoryImpl struct {
	engine *xorm.Engine
}

func NewProjectRepository(engine *xorm.Engine) *ProjectRepositoryImpl {
	return &ProjectRepositoryImpl{engine: engine}
}

func (r *ProjectRepositoryImpl) Create(ctx context.Context, environmentID int64, project *model.Project) error {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return err
	}
	if project == nil {
		return base.InvalidArgument("project is nil")
	}
	if project.EnvironmentID != 0 && project.EnvironmentID != environmentID {
		return base.InvalidArgument("project.environment_id does not match environment_id")
	}
	project.EnvironmentID = environmentID
	result, err := base.Execute(ctx, r.engine, "project", `
INSERT INTO projects
    (environment_id, `+"`key`"+`, name, description, created_by, updated_by)
SELECT e.id, ?, ?, ?, ?, ?
FROM environments AS e
WHERE e.id = ? AND e.deleted_at IS NULL`,
		project.Key, project.Name, project.Description, project.CreatedBy, project.UpdatedBy, environmentID)
	if err != nil {
		return err
	}
	project.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted project id", err)
	}
	return nil
}

func (r *ProjectRepositoryImpl) FindByID(ctx context.Context, environmentID, projectID int64) (*model.Project, error) {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return nil, err
	}
	var project model.Project
	err := base.FindOne(ctx, r.engine, "project", `
SELECT p.*
FROM projects AS p
WHERE p.environment_id = ? AND p.id = ? AND p.deleted_at IS NULL
LIMIT 1`, []any{environmentID, projectID}, &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *ProjectRepositoryImpl) FindByKey(ctx context.Context, environmentID int64, key string) (*model.Project, error) {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return nil, base.InvalidArgument("project key is empty")
	}
	var project model.Project
	err := base.FindOne(ctx, r.engine, "project", `
SELECT p.*
FROM projects AS p
WHERE p.environment_id = ? AND p.`+"`key`"+` = ? AND p.deleted_at IS NULL
LIMIT 1`, []any{environmentID, key}, &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *ProjectRepositoryImpl) List(ctx context.Context, environmentID int64, keyword string, page base.PageRequest) (base.PageResult[model.Project], error) {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return base.PageResult[model.Project]{}, err
	}
	page = base.NormalizePage(page)
	pattern := "%" + keyword + "%"
	total, err := base.Count(ctx, r.engine, "projects", `
SELECT COUNT(*) AS total
FROM projects AS p
WHERE p.environment_id = ? AND p.deleted_at IS NULL
  AND (p.name LIKE ? OR p.description LIKE ?)`, environmentID, pattern, pattern)
	if err != nil {
		return base.PageResult[model.Project]{}, err
	}
	result := base.PageResult[model.Project]{Total: total, Offset: page.Offset, Limit: page.Limit, Items: []model.Project{}}
	if total == 0 {
		return result, nil
	}
	err = base.FindAll(ctx, r.engine, "projects", `
SELECT p.*
FROM projects AS p
WHERE p.environment_id = ? AND p.deleted_at IS NULL
  AND (p.name LIKE ? OR p.description LIKE ?)
ORDER BY p.id DESC
LIMIT ? OFFSET ?`, []any{environmentID, pattern, pattern, page.Limit, page.Offset}, &result.Items)
	return result, err
}

func (r *ProjectRepositoryImpl) Update(ctx context.Context, environmentID, projectID int64, update ProjectUpdate) error {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return err
	}
	if update.Version < 0 {
		return base.InvalidArgument("version must not be negative")
	}
	_, err := base.Execute(ctx, r.engine, "project", `
UPDATE projects
SET name = ?, description = ?, updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE environment_id = ? AND id = ? AND version = ? AND deleted_at IS NULL`,
		update.Name, update.Description, update.UpdatedBy, environmentID, projectID, update.Version)
	return err
}

func (r *ProjectRepositoryImpl) UpdateTx(ctx context.Context, tx *xorm.Session, environmentID, projectID int64, update ProjectUpdate) error {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return err
	}
	if update.Version < 0 {
		return base.InvalidArgument("version must not be negative")
	}
	_, err := base.ExecuteTx(ctx, tx, "project", `
UPDATE projects
SET name = ?, description = ?, updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE environment_id = ? AND id = ? AND version = ? AND deleted_at IS NULL`,
		update.Name, update.Description, update.UpdatedBy, environmentID, projectID, update.Version)
	return err
}

func (r *ProjectRepositoryImpl) LockByID(ctx context.Context, tx *xorm.Session, environmentID, projectID int64) (*model.Project, error) {
	if err := validateEnvironmentResource(environmentID, "project_id", projectID); err != nil {
		return nil, err
	}
	var project model.Project
	err := base.LockOne(ctx, tx, "project", `
SELECT p.*
FROM projects AS p
WHERE p.environment_id = ? AND p.id = ? AND p.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{environmentID, projectID}, &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func validateEnvironmentResource(environmentID int64, resourceName string, resourceID int64) error {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return err
	}
	return base.ValidateID(resourceName, resourceID)
}

var _ ProjectRepository = (*ProjectRepositoryImpl)(nil)
