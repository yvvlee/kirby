package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type environmentLogic interface {
	VerifyProject(ctx context.Context, environmentID, projectID int64) error
	List(ctx context.Context, actor permission.Actor) ([]model.Environment, error)
	Create(ctx context.Context, actor permission.Actor, environment *model.Environment) (*model.Environment, error)
	Update(ctx context.Context, actor permission.Actor, environmentID int64, update repository.EnvironmentUpdate) (*model.Environment, error)
	Permissions(ctx context.Context, actor permission.Actor, environmentID int64) ([]string, error)
	ListMembers(ctx context.Context, actor permission.Actor, environmentID int64) ([]repository.EnvironmentMemberRecord, error)
	UpdateMemberRoles(ctx context.Context, actor permission.Actor, environmentID, userID int64, roleIDs []int64) error
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
	result := make([]*commonv1.Environment, 0, len(items))
	for index := range items {
		converted, err := converter.EnvironmentToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListEnvironmentsReply{List: result}, nil
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
	created, err := s.environmentLogic.Create(ctx, actor, &model.Environment{
		ProjectID:   request.ProjectId,
		Key:         request.Key,
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := converter.EnvironmentToProto(created)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.EnvironmentReply{Environment: converted}, nil
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
	updated, err := s.environmentLogic.Update(ctx, actor, request.EnvironmentId, repository.EnvironmentUpdate{
		Name:        request.Name,
		Description: request.Description,
		Enabled:     request.Enabled,
		Version:     int64(request.Version),
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := converter.EnvironmentToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.EnvironmentReply{Environment: converted}, nil
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

// ListEnvironmentUsers uses the shared account and role projections for environment members.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *EnvironmentService) ListEnvironmentUsers(ctx context.Context, request *adminv1.EnvironmentIDRequest) (*adminv1.ListEnvironmentUsersReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	members, err := s.environmentLogic.ListMembers(ctx, actor, request.EnvironmentId)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.EnvironmentMember, 0, len(members))
	for index := range members {
		member, err := converter.EnvironmentMemberToProto(&members[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, member)
	}
	return &adminv1.ListEnvironmentUsersReply{List: result}, nil
}

// UpdateEnvironmentUserRoles forwards the complete validated replacement role set.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: an empty success response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *EnvironmentService) UpdateEnvironmentUserRoles(ctx context.Context, request *adminv1.UpdateEnvironmentUserRolesRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.environmentLogic.UpdateMemberRoles(ctx, actor, request.EnvironmentId, request.UserId, request.RoleIds); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func filterEnvironments(items []model.Environment, projectID int64, scoped bool) []model.Environment {
	if !scoped {
		return items
	}
	filtered := make([]model.Environment, 0, len(items))
	for _, item := range items {
		if item.ProjectID == projectID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
