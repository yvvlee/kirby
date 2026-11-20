package repository

import (
	"context"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type ApplicationSettingsRepository interface {
	Get(ctx context.Context) (*model.ApplicationSettings, error)
	UpdateSelfRegistration(ctx context.Context, enabled bool, actorID int64) error
}

type ApplicationSettingsRepositoryImpl struct {
	*lorm.Repository[*model.ApplicationSettings]
}

// NewApplicationSettingsRepository uses the shared database to keep settings consistent across servers.
// Parameters: dependencies provide persistence and authorization.
// Returns: the configured component.
// Throws: none; errors are returned.
func NewApplicationSettingsRepository(storage *infra.Storage) *ApplicationSettingsRepositoryImpl {
	return &ApplicationSettingsRepositoryImpl{Repository: storage.CoreDB.Repository[*model.ApplicationSettings]()}
}

// Get reads the persisted registration policy rather than trusting client state.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (r *ApplicationSettingsRepositoryImpl) Get(ctx context.Context) (*model.ApplicationSettings, error) {
	item, found, err := r.Engine.Query[*model.ApplicationSettings]().ID(1).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("application settings")
	}
	return item, nil
}

// UpdateSelfRegistration persists the registration switch and its actor.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (r *ApplicationSettingsRepositoryImpl) UpdateSelfRegistration(ctx context.Context, enabled bool, actorID int64) error {
	fields := new(model.ApplicationSettings).LormCols()
	_, err := r.Engine.Update[*model.ApplicationSettings]().ID(1).SetMap(map[string]any{
		fields.AllowSelfRegistration(): enabled,
		fields.UpdatedBy():             actorID,
	}).Where(builder.Eq{fields.ID(): 1}).Exec(ctx)
	return WrapDatabaseError(err)
}

var _ ApplicationSettingsRepository = (*ApplicationSettingsRepositoryImpl)(nil)
