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
	Create(ctx context.Context, environment *model.Environment, audit *model.AuditLog) error
	Update(ctx context.Context, environmentID int64, update EnvironmentUpdate, audit *model.AuditLog) error
}

type EnvironmentRepositoryImpl struct {
	*lorm.Repository[*model.Environment]
	audits AuditLogRepository
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
	}
}

// Create creates an enabled environment and its mandatory audit in one transaction.
//
// Parameters: ctx carries the transaction; environment supplies project ownership and fields; audit records the creation.
// Returns: nil after insertion and audit, or a database or audit error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) Create(ctx context.Context, environment *model.Environment, audit *model.AuditLog) error {
	environment.PrepareInsert()
	environment.Enabled = true
	if _, err := r.Insert(ctx, environment); err != nil {
		return WrapDatabaseError(err)
	}
	return r.recordAudit(ctx, environment.ID, audit)
}

// Update guards environment edits with the expected version and records the audit atomically.
//
// Parameters: ctx carries the transaction; environmentID selects the row; update supplies fields, actor, and expected version; audit records the edit.
// Returns: nil after the guarded write and audit, ErrNoRowsAffected on a missing or stale row, or a persistence error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) Update(ctx context.Context, environmentID int64, update EnvironmentUpdate, audit *model.AuditLog) error {
	fields := new(model.Environment).LormCols()
	affected, err := r.Engine.Update[*model.Environment]().ID(environmentID).
		Where(builder.And{
			builder.Eq{
				fields.Version(): update.Version,
			},
			builder.IsNull(fields.DeletedAt()),
		}).
		SetMap(map[string]any{
			fields.Name():        update.Name,
			fields.Description(): update.Description,
			fields.Enabled():     update.Enabled,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now().UTC(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("environment")
	}
	return r.recordAudit(ctx, environmentID, audit)
}

// List returns live environments in stable ID order for administration.
//
// Parameters: ctx controls the database read.
// Returns: live environments ordered by ID, or a database error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) List(ctx context.Context) ([]model.Environment, error) {
	fields := new(model.Environment).LormCols()
	environments, err := r.Engine.Query[*model.Environment]().
		Where(builder.IsNull(fields.DeletedAt())).Asc(fields.ID()).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(environments), nil
}

// FindByID excludes deleted environments from subsequent authorization and editing.
//
// Parameters: ctx controls the read; environmentID identifies the environment.
// Returns: the live environment, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *EnvironmentRepositoryImpl) FindByID(ctx context.Context, environmentID int64) (*model.Environment, error) {
	fields := new(model.Environment).LormCols()
	environment, found, err := r.Engine.Query[*model.Environment]().ID(environmentID).
		Where(builder.IsNull(fields.DeletedAt())).Get(ctx)
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
	return r.audits.RecordForEnvironment(ctx, environmentID, audit)
}

var _ EnvironmentRepository = (*EnvironmentRepositoryImpl)(nil)
