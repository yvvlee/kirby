package structure

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/structure"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type Logic interface {
	Create(context.Context, permission.Actor, int64, int64, string, string, string) (*model.Structure, error)
	Update(context.Context, permission.Actor, int64, int64, string, string, string, []*commonv1.Field, int64) (*model.Structure, error)
	List(context.Context, permission.Actor, int64, int64, int64, *int64) ([]*commonv1.Structure, error)
	Delete(context.Context, permission.Actor, int64, int64) error
}
type Service struct{ logic Logic }

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("structure service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.StructureServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateStructure(ctx context.Context, request *adminv1.CreateStructureRequest) (*adminv1.StructureReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Create(ctx, actor, request.EnvironmentId, request.ConfigId, request.Key, request.Name, request.Description)
	return reply(item, err)
}

func (s *Service) UpdateStructure(ctx context.Context, request *adminv1.UpdateStructureRequest) (*adminv1.StructureReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Update(ctx, actor, request.EnvironmentId, request.Id, request.Key, request.Name, request.Description, request.Fields, int64(request.Version))
	return reply(item, err)
}

func (s *Service) ListStructure(ctx context.Context, request *adminv1.ListStructureRequest) (*adminv1.ListStructureReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	items, err := s.logic.List(ctx, actor, request.EnvironmentId, request.ProjectId, request.ConfigId, request.IgnoreDependencyId)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.ListStructureReply{List: items}, nil
}

func (s *Service) DeleteStructure(ctx context.Context, request *adminv1.StructureIDRequest) (*emptypb.Empty, error) {
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

func reply(item *model.Structure, err error) (*adminv1.StructureReply, error) {
	if err != nil {
		return nil, entity.APIError(err)
	}
	converted, err := converter.StructureToProto(item)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.StructureReply{Structure: converted}, nil
}

func badRequest() error { return entity.APIError(entity.Invalid("invalid request")) }
