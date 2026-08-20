package repository

import (
	"context"
	"strconv"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type EnvironmentUpdate struct {
	Name        string
	Description string
	Enabled     bool
	UpdatedBy   int64
	Version     int64
}

type EnvironmentRepository interface {
	List(context.Context) ([]model.Environment, error)
	FindByID(context.Context, int64) (*model.Environment, error)
	Create(context.Context, *model.Environment, *model.AuditLog) error
	Update(context.Context, int64, EnvironmentUpdate, *model.AuditLog) error
}

type EnvironmentRepositoryImpl struct {
	engine *xorm.Engine
	audits AuditLogRepository
}

func NewEnvironmentRepository(engine *xorm.Engine, audits AuditLogRepository) *EnvironmentRepositoryImpl {
	return &EnvironmentRepositoryImpl{engine: engine, audits: audits}
}

func (r *EnvironmentRepositoryImpl) List(ctx context.Context) ([]model.Environment, error) {
	environments := make([]model.Environment, 0)
	err := base.FindAll(ctx, r.engine, "environments", `
SELECT e.*
FROM environments AS e
WHERE e.deleted_at IS NULL
ORDER BY e.id ASC`, nil, &environments)
	return environments, err
}

func (r *EnvironmentRepositoryImpl) FindByID(ctx context.Context, environmentID int64) (*model.Environment, error) {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return nil, err
	}
	var environment model.Environment
	err := base.FindOne(ctx, r.engine, "environment", `
SELECT e.*
FROM environments AS e
WHERE e.id = ? AND e.deleted_at IS NULL
LIMIT 1`, []any{environmentID}, &environment)
	return &environment, err
}

func (r *EnvironmentRepositoryImpl) Create(ctx context.Context, environment *model.Environment, audit *model.AuditLog) error {
	if environment == nil || audit == nil || r.audits == nil {
		return base.InvalidArgument("environment and audit log are required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		if err := base.ValidateID("project_id", environment.ProjectID); err != nil {
			return err
		}
		result, err := base.ExecuteTx(ctx, tx, "environment", `
INSERT INTO environments
    (project_id, `+"`key`"+`, name, description, enabled, created_by, updated_by)
VALUES (?, ?, ?, ?, TRUE, ?, ?)`, environment.ProjectID, environment.Key, environment.Name, environment.Description, environment.CreatedBy, environment.UpdatedBy)
		if err != nil {
			return err
		}
		environment.ID, err = result.LastInsertId()
		if err != nil {
			return base.Wrap("read inserted environment id", err)
		}
		audit.ResourceID = strconv.FormatInt(environment.ID, 10)
		return r.audits.RecordForEnvironmentTx(ctx, tx, environment.ID, audit)
	})
}

func (r *EnvironmentRepositoryImpl) Update(ctx context.Context, environmentID int64, update EnvironmentUpdate, audit *model.AuditLog) error {
	if err := base.ValidateID("environment_id", environmentID); err != nil {
		return err
	}
	if update.Version < 0 || audit == nil || r.audits == nil {
		return base.InvalidArgument("valid version and audit log are required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		_, err := base.ExecuteTx(ctx, tx, "environment", `
UPDATE environments
SET name = ?, description = ?, enabled = ?, updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND version = ? AND deleted_at IS NULL`,
			update.Name, update.Description, update.Enabled, update.UpdatedBy, environmentID, update.Version)
		if err != nil {
			return err
		}
		audit.ResourceID = strconv.FormatInt(environmentID, 10)
		return r.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit)
	})
}

var _ EnvironmentRepository = (*EnvironmentRepositoryImpl)(nil)
