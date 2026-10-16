package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

// ErrAlreadyInitialized prevents the public setup endpoint from reopening.
var ErrAlreadyInitialized = errors.New("installation administrator already initialized")

// SetupRepository persists the installation's permanent initialization state.
type SetupRepository interface {
	IsInitialized(ctx context.Context) (bool, error)
	CreateInitialAdmin(ctx context.Context, user *model.User) error
	MarkInitialized(ctx context.Context) error
}

// SetupRepositoryImpl serializes administrator initialization across instances.
type SetupRepositoryImpl struct {
	*lorm.Repository[*model.ApplicationSecrets]
}

// NewSetupRepository shares the database transaction used by the caller.
//
// Parameters: storage supplies the relational database.
// Returns: the installation setup repository.
// Throws: none.
func NewSetupRepository(storage *infra.Storage) *SetupRepositoryImpl {
	return &SetupRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.ApplicationSecrets](),
	}
}

// IsInitialized reads the database so other instances observe completion immediately.
//
// Parameters: ctx is the request context.
// Returns: whether administrator initialization has completed, or a database error.
// Throws: none.
func (r *SetupRepositoryImpl) IsInitialized(ctx context.Context) (bool, error) {
	state, err := r.loadState(ctx, false)
	if err != nil {
		return false, err
	}
	return state.AdminInitialized, nil
}

// CreateInitialAdmin atomically claims setup within the caller's transaction.
//
// Parameters: ctx carries the transaction; user contains the prepared administrator.
// Returns: ErrAlreadyInitialized if claimed, or a database error.
// Throws: none.
func (r *SetupRepositoryImpl) CreateInitialAdmin(ctx context.Context, user *model.User) error {
	state, err := r.loadState(ctx, true)
	if err != nil {
		return err
	}
	if state.AdminInitialized {
		return ErrAlreadyInitialized
	}
	user.PrepareInsert()
	if _, err := r.Engine.Insert[*model.User]().AddModel(user).Exec(ctx); err != nil {
		return fmt.Errorf("create initial administrator: %w", err)
	}
	return r.markInitialized(ctx)
}

// MarkInitialized takes the same lock as web setup before the CLI changes any user.
//
// Parameters: ctx carries the administrator provisioning transaction.
// Returns: a database error if the installation state cannot be persisted.
// Throws: none.
func (r *SetupRepositoryImpl) MarkInitialized(ctx context.Context) error {
	state, err := r.loadState(ctx, true)
	if err != nil {
		return err
	}
	if state.AdminInitialized {
		return nil
	}
	return r.markInitialized(ctx)
}

func (r *SetupRepositoryImpl) loadState(ctx context.Context, lock bool) (*model.ApplicationSecrets, error) {
	fields := new(model.ApplicationSecrets).LormCols()
	query := r.Engine.Query[*model.ApplicationSecrets]().Select(fields.ID(), fields.AdminInitialized()).ID(1)
	if lock {
		query.Suffix("FOR UPDATE")
	}
	state, found, err := query.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("read installation initialization state: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("installation initialization state is missing")
	}
	return state, nil
}

func (r *SetupRepositoryImpl) markInitialized(ctx context.Context) error {
	affected, err := r.Engine.Update[*model.ApplicationSecrets]().ID(1).
		Set(new(model.ApplicationSecrets).LormCols().AdminInitialized(), true).Exec(ctx)
	if err != nil {
		return fmt.Errorf("complete administrator initialization: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("complete administrator initialization: %w", ErrNoRowsAffected)
	}
	return nil
}
