package project

import (
	"context"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/project"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type Logic interface {
	Create(context.Context, permission.Actor, int64, *model.Project) (*model.Project, error)
	Update(context.Context, permission.Actor, int64, int64, repository.ProjectUpdate) (*model.Project, error)
	List(context.Context, permission.Actor, int64, string) ([]model.Project, error)
	Detail(context.Context, permission.Actor, int64, int64) (*model.Project, error)
}

type Service struct{ logic Logic }

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("project service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.ProjectServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateProject(ctx context.Context, request *adminv1.CreateProjectRequest) (*adminv1.ProjectReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, entity.APIError(entity.Invalid("invalid request"))
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Create(ctx, actor, request.EnvironmentId, &model.Project{Key: request.Key, Name: request.Name, Description: request.Description})
	return projectReply(item, err)
}

func (s *Service) UpdateProject(ctx context.Context, request *adminv1.UpdateProjectRequest) (*adminv1.ProjectReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, entity.APIError(entity.Invalid("invalid request"))
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Update(ctx, actor, request.EnvironmentId, request.Id, repository.ProjectUpdate{Name: request.Name, Description: request.Description, Version: int64(request.Version)})
	return projectReply(item, err)
}

func (s *Service) ListProject(ctx context.Context, request *adminv1.ListProjectRequest) (*adminv1.ListProjectReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, entity.APIError(entity.Invalid("invalid request"))
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	items, err := s.logic.List(ctx, actor, request.EnvironmentId, request.Keyword)
	if err != nil {
		return nil, entity.APIError(err)
	}
	result := make([]*commonv1.Project, 0, len(items))
	for index := range items {
		converted, err := converter.ProjectToProto(&items[index])
		if err != nil {
			return nil, entity.APIError(err)
		}
		result = append(result, converted)
	}
	return &adminv1.ListProjectReply{List: result}, nil
}

func (s *Service) ProjectDetail(ctx context.Context, request *adminv1.ProjectIDRequest) (*adminv1.ProjectReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, entity.APIError(entity.Invalid("invalid request"))
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Detail(ctx, actor, request.EnvironmentId, request.Id)
	return projectReply(item, err)
}

func projectReply(item *model.Project, err error) (*adminv1.ProjectReply, error) {
	if err != nil {
		return nil, entity.APIError(err)
	}
	converted, err := converter.ProjectToProto(item)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.ProjectReply{Project: converted}, nil
}
