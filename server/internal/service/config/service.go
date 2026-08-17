package config

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/config"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type Logic interface {
	Create(context.Context, permission.Actor, int64, int64, string, string) (*model.Config, error)
	Update(context.Context, permission.Actor, int64, int64, string, *commonv1.Field_Type, bool, int64) (*model.Config, error)
	UpdateValue(context.Context, permission.Actor, int64, int64, string, int64) (*model.Config, error)
	List(context.Context, permission.Actor, int64, repository.ConfigFilter) ([]model.Config, map[int64]bool, error)
	Delete(context.Context, permission.Actor, int64, int64) error
	Detail(context.Context, permission.Actor, int64, int64) (*model.Config, *commonv1.TreeNode, error)
}

type Service struct{ logic Logic }

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("config service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.ConfigServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateConfig(ctx context.Context, request *adminv1.CreateConfigRequest) (*adminv1.ConfigReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Create(ctx, actor, request.EnvironmentId, request.ProjectId, request.Key, request.Description)
	return configReply(item, err)
}

func (s *Service) UpdateConfig(ctx context.Context, request *adminv1.UpdateConfigRequest) (*adminv1.ConfigReply, error) {
	if request == nil || request.ValidateAll() != nil || request.Type == nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Update(ctx, actor, request.EnvironmentId, request.Id, request.Description, request.Type, request.IsArray, int64(request.Version))
	return configReply(item, err)
}

func (s *Service) UpdateConfigValue(ctx context.Context, request *adminv1.UpdateConfigValueRequest) (*adminv1.ConfigReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.UpdateValue(ctx, actor, request.EnvironmentId, request.Id, request.Value, int64(request.Version))
	return configReply(item, err)
}

func (s *Service) ListConfig(ctx context.Context, request *adminv1.ListConfigRequest) (*adminv1.ListConfigReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	items, released, err := s.logic.List(ctx, actor, request.EnvironmentId, repository.ConfigFilter{ProjectID: request.ProjectId, ProjectKey: request.ProjectKey, Key: request.Key})
	if err != nil {
		return nil, entity.APIError(err)
	}
	result := make([]*adminv1.ListConfigItem, 0, len(items))
	for index := range items {
		converted, err := converter.ConfigToProto(&items[index])
		if err != nil {
			return nil, entity.APIError(err)
		}
		result = append(result, &adminv1.ListConfigItem{Config: converted, IsReleased: released[items[index].ID]})
	}
	return &adminv1.ListConfigReply{List: result}, nil
}

func (s *Service) DeleteConfig(ctx context.Context, request *adminv1.ConfigIDRequest) (*emptypb.Empty, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	if err := s.logic.Delete(ctx, actor, request.EnvironmentId, request.Id); err != nil {
		return nil, entity.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) ConfigDetail(ctx context.Context, request *adminv1.ConfigIDRequest) (*adminv1.ConfigDetailReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, tree, err := s.logic.Detail(ctx, actor, request.EnvironmentId, request.Id)
	if err != nil {
		return nil, entity.APIError(err)
	}
	converted, err := converter.ConfigToProto(item)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.ConfigDetailReply{Config: converted, Tree: tree}, nil
}

func badRequest() error { return entity.APIError(entity.Invalid("invalid request")) }

func configReply(item *model.Config, err error) (*adminv1.ConfigReply, error) {
	if err != nil {
		return nil, entity.APIError(err)
	}
	converted, err := converter.ConfigToProto(item)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.ConfigReply{Config: converted}, nil
}
