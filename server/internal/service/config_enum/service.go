package configenum

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/config_enum"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type Logic interface {
	Create(context.Context, permission.Actor, int64, int64, string, string, string, []*commonv1.SelectOption) (*model.ConfigEnum, error)
	Update(context.Context, permission.Actor, int64, int64, string, string, string, []*commonv1.SelectOption, int64) (*model.ConfigEnum, error)
	List(context.Context, permission.Actor, int64, int64, int64) ([]model.ConfigEnum, error)
	Delete(context.Context, permission.Actor, int64, int64) error
}
type Service struct{ logic Logic }

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("config enum service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.EnumServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateEnum(ctx context.Context, request *adminv1.CreateEnumRequest) (*adminv1.EnumReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Create(ctx, actor, request.EnvironmentId, request.ConfigId, request.Key, request.Name, request.Description, request.Values)
	return reply(item, err)
}

func (s *Service) UpdateEnum(ctx context.Context, request *adminv1.UpdateEnumRequest) (*adminv1.EnumReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Update(ctx, actor, request.EnvironmentId, request.Id, request.Key, request.Name, request.Description, request.Values, int64(request.Version))
	return reply(item, err)
}

func (s *Service) ListEnum(ctx context.Context, request *adminv1.ListEnumRequest) (*adminv1.ListEnumReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	items, err := s.logic.List(ctx, actor, request.EnvironmentId, request.ProjectId, request.ConfigId)
	if err != nil {
		return nil, entity.APIError(err)
	}
	result := make([]*commonv1.ConfigEnum, 0, len(items))
	for index := range items {
		converted, err := converter.EnumToProto(&items[index])
		if err != nil {
			return nil, entity.APIError(err)
		}
		result = append(result, converted)
	}
	return &adminv1.ListEnumReply{List: result}, nil
}

func (s *Service) DeleteEnum(ctx context.Context, request *adminv1.EnumIDRequest) (*emptypb.Empty, error) {
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

func reply(item *model.ConfigEnum, err error) (*adminv1.EnumReply, error) {
	if err != nil {
		return nil, entity.APIError(err)
	}
	converted, err := converter.EnumToProto(item)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.EnumReply{Enum: converted}, nil
}

func badRequest() error { return entity.APIError(entity.Invalid("invalid request")) }
