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
	CreateInitialAdmin(ctx context.Context, user *model.User) (rowsAffected int64, err error)
	MarkInitialized(ctx context.Context) (rowsAffected int64, err error)
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
// Returns: rowsAffected from the initialization marker write, ErrAlreadyInitialized if claimed, or a database error.
// Throws: none.
func (r *SetupRepositoryImpl) CreateInitialAdmin(ctx context.Context, user *model.User) (rowsAffected int64, err error) {
	state, err := r.loadState(ctx, true)
	if err != nil {
		return
	}
	if state.AdminInitialized {
		err = ErrAlreadyInitialized
		return
	}
	if _, err = r.Engine.Insert[*model.User]().AddModel(user).Exec(ctx); err != nil {
		err = fmt.Errorf("create initial administrator: %w", err)
		return
	}
	rowsAffected, err = r.markInitialized(ctx)
	return
}

// MarkInitialized takes the same lock as web setup before the CLI changes any user.
//
// Parameters: ctx carries the administrator provisioning transaction.
// Returns: rowsAffected from the initialization marker write, 0 when already initialized, or a database error.
// Throws: none.
func (r *SetupRepositoryImpl) MarkInitialized(ctx context.Context) (rowsAffected int64, err error) {
	state, err := r.loadState(ctx, true)
	if err != nil {
		return
	}
	if state.AdminInitialized {
		return
	}
	rowsAffected, err = r.markInitialized(ctx)
	return
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

func (r *SetupRepositoryImpl) markInitialized(ctx context.Context) (rowsAffected int64, err error) {
	rowsAffected, err = r.Engine.Update[*model.ApplicationSecrets]().ID(1).
		Set(new(model.ApplicationSecrets).LormCols().AdminInitialized(), true).Exec(ctx)
	if err != nil {
		err = fmt.Errorf("complete administrator initialization: %w", err)
		return
	}
	if rowsAffected != 1 {
		err = fmt.Errorf("complete administrator initialization: %w", ErrNoRowsAffected)
		return
	}
	return
}
