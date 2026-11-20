package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type EnvironmentUpdate struct {
	Name        string
	Description string
	Enabled     bool
	UpdatedBy   int64
	Version     int64
}

type EnvironmentRepository interface {
	List(ctx context.Context) ([]model.Environment, error)
	FindByID(ctx context.Context, environmentID int64) (*model.Environment, error)
	Create(ctx context.Context, environment *model.Environment, audit *model.AuditLog) (rowsAffected int64, err error)
	Update(ctx context.Context, environmentID int64, update EnvironmentUpdate, audit *model.AuditLog) (rowsAffected int64, err error)
}

type EnvironmentRepositoryImpl struct {
	*lorm.Repository[*model.Environment]
	audits AuditLogRepository
	grants *lorm.Repository[*model.EnvironmentPermissionGrant]
}

// NewEnvironmentRepository binds the EnvironmentRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured EnvironmentRepository.
// Throws: none.
func NewEnvironmentRepository(storage *infra.Storage) *EnvironmentRepositoryImpl {
	return &EnvironmentRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Environment](),
		audits:     NewAuditLogRepository(storage),
		grants:     storage.CoreDB.Repository[*model.EnvironmentPermissionGrant](),
	}
}

// Create creates an enabled environment and its mandatory audit in one transaction.
//
// Parameters: ctx carries the transaction; environment supplies project ownership and fields; audit records the creation.
// Returns: rowsAffected from the environment insert after the audit write, or a database or audit error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) Create(ctx context.Context, environment *model.Environment, audit *model.AuditLog) (rowsAffected int64, err error) {
	environment.Enabled = true
	rowsAffected, err = r.Insert(ctx, environment)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	err = r.recordAudit(ctx, environment.ID, audit)
	if err != nil {
		return
	}
	for _, role := range []string{"OWNER", "MAINTAINER", "MEMBER", "GUEST"} {
		permissions := []string{"READ"}
		if role == "OWNER" || role == "MAINTAINER" {
			permissions = []string{"READ", "WRITE", "PUBLISH", "API_KEY_MANAGE"}
		}
		if role == "MEMBER" {
			permissions = []string{"READ", "WRITE"}
		}
		if err = r.createRoleGrants(ctx, environment, role, permissions); err != nil {
			return
		}
	}
	return
}

// Update guards environment edits with the expected version and records the audit atomically.
//
// Parameters: ctx carries the transaction; environmentID selects the row; update supplies fields, actor, and expected version; audit records the edit.
// Returns: rowsAffected from the guarded environment write after the audit, ErrNoRowsAffected on a missing or stale row, or a persistence error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) Update(ctx context.Context, environmentID int64, update EnvironmentUpdate, audit *model.AuditLog) (rowsAffected int64, err error) {
	fields := new(model.Environment).LormCols()
	rowsAffected, err = r.Engine.Update[*model.Environment]().ID(environmentID).
		Where(builder.Eq{
			fields.Version(): update.Version,
		}).
		SetMap(map[string]any{
			fields.Name():        update.Name,
			fields.Description(): update.Description,
			fields.Enabled():     update.Enabled,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("environment")
		return
	}
	err = r.recordAudit(ctx, environmentID, audit)
	return
}

// List returns environments in stable ID order for administration.
//
// Parameters: ctx controls the database read.
// Returns: environments ordered by ID, or a database error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) List(ctx context.Context) ([]model.Environment, error) {
	fields := new(model.Environment).LormCols()
	environments, err := r.Engine.Query[*model.Environment]().
		Asc(fields.ID()).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(environments), nil
}

// FindByID loads the environment for authorization and editing.
//
// Parameters: ctx controls the read; environmentID identifies the environment.
// Returns: the environment, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) FindByID(ctx context.Context, environmentID int64) (*model.Environment, error) {
	environment, found, err := r.Engine.Query[*model.Environment]().ID(environmentID).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("environment")
	}
	return environment, nil
}

func (r *EnvironmentRepositoryImpl) recordAudit(ctx context.Context, environmentID int64, audit *model.AuditLog) error {
	audit.ResourceID = strconv.FormatInt(environmentID, 10)
	_, err := r.audits.RecordForEnvironment(ctx, environmentID, audit)
	return err
}

var _ EnvironmentRepository = (*EnvironmentRepositoryImpl)(nil)

func (r *EnvironmentRepositoryImpl) createRoleGrants(ctx context.Context, environment *model.Environment, role string, permissions []string) error {
	for _, permission := range permissions {
		if _, insertErr := r.grants.Insert(ctx, &model.EnvironmentPermissionGrant{
			EnvironmentID: environment.ID,
			Role:          role,
			Permission:    permission,
			CreatedBy:     environment.CreatedBy,
		}); insertErr != nil {
			return WrapDatabaseError(insertErr)
		}
	}
	return nil
}
