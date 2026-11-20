package service

import (
	"context"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type SystemSettingsService struct{ logic *logic.SystemSettingsLogic }

// NewSystemSettingsService binds settings endpoints to authorized operations.
// Parameters: dependencies provide persistence and authorization.
// Returns: the configured component.
// Throws: none; errors are returned.
func NewSystemSettingsService(logicLayer *logic.SystemSettingsLogic) *SystemSettingsService {
	return &SystemSettingsService{logic: logicLayer}
}

// GetSystemSettings checks the authenticated administrator before disclosing settings.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (s *SystemSettingsService) GetSystemSettings(ctx context.Context, _ *adminv1.GetSystemSettingsRequest) (*adminv1.SystemSettingsReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	enabled, err := s.logic.Get(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.SystemSettingsReply{AllowSelfRegistration: enabled}, nil
}

// UpdateSystemSettings checks administrator authority before updating the registration switch.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (s *SystemSettingsService) UpdateSystemSettings(ctx context.Context, req *adminv1.UpdateSystemSettingsRequest) (*adminv1.SystemSettingsReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	enabled, err := s.logic.Update(ctx, actor, req.AllowSelfRegistration)
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.SystemSettingsReply{AllowSelfRegistration: enabled}, nil
}

// GetRegistrationStatus returns the public registration switch without exposing privileged settings.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (s *SystemSettingsService) GetRegistrationStatus(ctx context.Context, _ *adminv1.GetSystemSettingsRequest) (*adminv1.SystemSettingsReply, error) {
	enabled, err := s.logic.RegistrationStatus(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SystemSettingsReply{AllowSelfRegistration: enabled}, nil
}
