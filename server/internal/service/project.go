package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type projectLogic interface {
	Delete(ctx context.Context, actor permission.Actor, projectID int64) error
	Create(ctx context.Context, param *entity.CreateProjectParam) (*commonv1.Project, error)
	Update(ctx context.Context, param *entity.UpdateProjectParam) (*commonv1.Project, error)
	List(ctx context.Context, param *entity.ListProjectParam) ([]*commonv1.Project, error)
	Detail(ctx context.Context, actor permission.Actor, environmentID, projectID int64) (*commonv1.Project, error)
}

type ProjectService struct{ projectLogic projectLogic }

// NewProjectService creates ProjectService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *ProjectService.
// Throws: none.
func NewProjectService(logicLayer *logic.ProjectLogic) *ProjectService {
	return &ProjectService{projectLogic: logicLayer}
}

var _ adminv1.ProjectServiceHTTPServer = (*ProjectService)(nil)

// CreateProject maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ProjectReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ProjectService) CreateProject(ctx context.Context, request *adminv1.CreateProjectRequest) (*adminv1.ProjectReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.projectLogic.Create(ctx, &entity.CreateProjectParam{
		Actor:       actor,
		Key:         request.Key,
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ProjectReply{Project: item}, nil
}

// UpdateProject maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ProjectReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ProjectService) UpdateProject(ctx context.Context, request *adminv1.UpdateProjectRequest) (*adminv1.ProjectReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.projectLogic.Update(ctx, &entity.UpdateProjectParam{
		Actor:     actor,
		ProjectID: request.Id,
		Update: repository.ProjectUpdate{
			Name:        request.Name,
			Description: request.Description,
			Version:     int64(request.Version),
		},
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ProjectReply{Project: item}, nil
}

// ListProject maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ListProjectReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ProjectService) ListProject(ctx context.Context, request *adminv1.ListProjectRequest) (*adminv1.ListProjectReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	environmentID := int64(0)
	if request.EnvironmentId != nil {
		environmentID = *request.EnvironmentId
	}
	items, err := s.projectLogic.List(ctx, &entity.ListProjectParam{
		Actor:         actor,
		EnvironmentID: environmentID,
		Keyword:       request.Keyword,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ListProjectReply{List: items}, nil
}

// ProjectDetail maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ProjectReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ProjectService) ProjectDetail(ctx context.Context, request *adminv1.ProjectIDRequest) (*adminv1.ProjectReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	environmentID := int64(0)
	if request.EnvironmentId != nil {
		environmentID = *request.EnvironmentId
	}
	item, err := s.projectLogic.Detail(ctx, actor, environmentID, request.Id)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ProjectReply{Project: item}, nil
}

// DeleteProject removes only a project owned by the authenticated caller.
// Parameters: ctx carries authentication; request selects the project.
// Returns: an empty reply or a translated error.
// Throws: none; errors are returned.
func (s *ProjectService) DeleteProject(ctx context.Context, request *adminv1.ProjectIDRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	if err := s.projectLogic.Delete(ctx, actor, request.Id); err != nil {
		return nil, apiError(err)
	}
	return &emptypb.Empty{}, nil
}
