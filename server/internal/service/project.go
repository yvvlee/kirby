package service

import (
	"context"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type projectLogic interface {
	Create(ctx context.Context, actor permission.Actor, environmentID int64, item *model.Project) (*model.Project, error)
	Update(ctx context.Context, actor permission.Actor, environmentID, projectID int64, update repository.ProjectUpdate) (*model.Project, error)
	List(ctx context.Context, actor permission.Actor, environmentID int64, keyword string) ([]model.Project, error)
	Detail(ctx context.Context, actor permission.Actor, environmentID, projectID int64) (*model.Project, error)
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
	item, err := s.projectLogic.Create(ctx, actor, 0, &model.Project{
		Key:         request.Key,
		Name:        request.Name,
		Description: request.Description,
	})
	return projectReply(item, err)
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
	item, err := s.projectLogic.Update(ctx, actor, 0, request.Id, repository.ProjectUpdate{
		Name:        request.Name,
		Description: request.Description,
		Version:     int64(request.Version),
	})
	return projectReply(item, err)
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
	items, err := s.projectLogic.List(ctx, actor, environmentID, request.Keyword)
	if err != nil {
		return nil, apiError(err)
	}
	result := make([]*commonv1.Project, 0, len(items))
	for index := range items {
		converted, err := converter.ProjectToProto(&items[index])
		if err != nil {
			return nil, apiError(err)
		}
		result = append(result, converted)
	}
	return &adminv1.ListProjectReply{List: result}, nil
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
	return projectReply(item, err)
}

func projectReply(item *model.Project, err error) (*adminv1.ProjectReply, error) {
	if err != nil {
		return nil, apiError(err)
	}
	converted, err := converter.ProjectToProto(item)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ProjectReply{Project: converted}, nil
}
