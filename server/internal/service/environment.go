package service

import (
	"context"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type environmentLogic interface {
	VerifyProject(ctx context.Context, environmentID, projectID int64) error
	List(ctx context.Context, actor permission.Actor) ([]*commonv1.Environment, error)
	Create(ctx context.Context, param *entity.CreateEnvironmentParam) (*commonv1.Environment, error)
	Update(ctx context.Context, param *entity.UpdateEnvironmentParam) (*commonv1.Environment, error)
	Permissions(ctx context.Context, actor permission.Actor, environmentID int64) ([]string, error)
}

type EnvironmentService struct{ environmentLogic environmentLogic }

// NewEnvironmentService binds API handling to the Environment business operations.
//
// Parameters: logicLayer implements authorization and transactional business operations.
// Returns: the configured EnvironmentService.
// Throws: none.
func NewEnvironmentService(logicLayer *logic.EnvironmentLogic) *EnvironmentService {
	return &EnvironmentService{environmentLogic: logicLayer}
}

var _ adminv1.EnvironmentServiceHTTPServer = (*EnvironmentService)(nil)

// ListEnvironments filters visible environments by the optional project and uses the shared environment projection.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *EnvironmentService) ListEnvironments(ctx context.Context, request *adminv1.ListEnvironmentsRequest) (*adminv1.ListEnvironmentsReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.environmentLogic.List(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items = filterEnvironments(items, request.GetProjectId(), request.ProjectId != nil)
	return &adminv1.ListEnvironmentsReply{List: items}, nil
}

// CreateEnvironment preserves project ownership when creating an environment.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *EnvironmentService) CreateEnvironment(ctx context.Context, request *adminv1.CreateEnvironmentRequest) (*adminv1.EnvironmentReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	created, err := s.environmentLogic.Create(ctx, &entity.CreateEnvironmentParam{
		Actor:       actor,
		ProjectID:   request.ProjectId,
		Key:         request.Key,
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.EnvironmentReply{Environment: created}, nil
}

// UpdateEnvironment requires project ownership verification before forwarding any environment edit.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *EnvironmentService) UpdateEnvironment(ctx context.Context, request *adminv1.UpdateEnvironmentRequest) (*adminv1.EnvironmentReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.environmentLogic.VerifyProject(ctx, request.EnvironmentId, request.ProjectId); err != nil {
		return nil, permission.APIError(err)
	}
	updated, err := s.environmentLogic.Update(ctx, &entity.UpdateEnvironmentParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		Update: repository.EnvironmentUpdate{
			Name:        request.Name,
			Description: request.Description,
			Enabled:     request.Enabled,
			Version:     int64(request.Version),
		},
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.EnvironmentReply{Environment: updated}, nil
}

// MyPermissions returns the authenticated actor current permissions for the requested environment.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *EnvironmentService) MyPermissions(ctx context.Context, request *adminv1.EnvironmentIDRequest) (*adminv1.MyPermissionsReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	keys, err := s.environmentLogic.Permissions(ctx, actor, request.EnvironmentId)
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.MyPermissionsReply{Permissions: keys}, nil
}

func filterEnvironments(items []*commonv1.Environment, projectID int64, scoped bool) []*commonv1.Environment {
	if !scoped {
		return items
	}
	filtered := make([]*commonv1.Environment, 0, len(items))
	for _, item := range items {
		if item.ProjectId == projectID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
