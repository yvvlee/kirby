package logic

import (
	"context"

	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type SystemSettingsLogic struct {
	repo        repository.ApplicationSettingsRepository
	permissions userAuthorizer
}

// NewSystemSettingsLogic binds settings persistence to system administrator authorization.
// Parameters: dependencies provide persistence and authorization.
// Returns: the configured component.
// Throws: none; errors are returned.
func NewSystemSettingsLogic(repo repository.ApplicationSettingsRepository, permissions userAuthorizer) *SystemSettingsLogic {
	return &SystemSettingsLogic{
		repo:        repo,
		permissions: permissions,
	}
}

// Get reads the persisted registration policy rather than trusting client state.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (l *SystemSettingsLogic) Get(ctx context.Context, actor permission.Actor) (bool, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return false, err
	}
	item, err := l.repo.Get(ctx)
	if err != nil {
		return false, err
	}
	return item.AllowSelfRegistration, nil
}

// Update requires system administrator authority before changing registration policy.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (l *SystemSettingsLogic) Update(ctx context.Context, actor permission.Actor, enabled bool) (bool, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return false, err
	}
	if err := l.repo.UpdateSelfRegistration(ctx, enabled, actor.UserID); err != nil {
		return false, err
	}
	return enabled, nil
}

// RegistrationStatus exposes only the registration switch to anonymous clients.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (l *SystemSettingsLogic) RegistrationStatus(ctx context.Context) (bool, error) {
	item, err := l.repo.Get(ctx)
	if err != nil {
		return false, err
	}
	return item.AllowSelfRegistration, nil
}
