package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type enumLogic interface {
	Create(ctx context.Context, actor permission.Actor, environmentID int64, input logic.EnumCreateInput) (*model.ConfigEnum, error)
	Update(ctx context.Context, actor permission.Actor, environmentID int64, input logic.EnumUpdateInput) (*model.ConfigEnum, error)
	List(ctx context.Context, actor permission.Actor, environmentID, projectID, configID int64) ([]model.ConfigEnum, error)
	Delete(ctx context.Context, actor permission.Actor, environmentID, enumID int64) error
}
type EnumService struct{ enumLogic enumLogic }

// NewEnumService creates EnumService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *EnumService.
// Throws: none.
func NewEnumService(logicLayer *logic.EnumLogic) *EnumService {
	return &EnumService{enumLogic: logicLayer}
}

var _ adminv1.EnumServiceHTTPServer = (*EnumService)(nil)

// CreateEnum maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.EnumReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *EnumService) CreateEnum(ctx context.Context, request *adminv1.CreateEnumRequest) (*adminv1.EnumReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.enumLogic.Create(ctx, actor, request.EnvironmentId, logic.EnumCreateInput{
		ConfigID:    request.ConfigId,
		Key:         request.Key,
		Name:        request.Name,
		Description: request.Description,
		Values:      request.Values,
	})
	return enumReply(item, err)
}

// UpdateEnum maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.EnumReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *EnumService) UpdateEnum(ctx context.Context, request *adminv1.UpdateEnumRequest) (*adminv1.EnumReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.enumLogic.Update(ctx, actor, request.EnvironmentId, logic.EnumUpdateInput{
		EnumID:      request.Id,
		Key:         request.Key,
		Name:        request.Name,
		Description: request.Description,
		Values:      request.Values,
		Version:     int64(request.Version),
	})
	return enumReply(item, err)
}

// ListEnum maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ListEnumReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *EnumService) ListEnum(ctx context.Context, request *adminv1.ListEnumRequest) (*adminv1.ListEnumReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	items, err := s.enumLogic.List(ctx, actor, request.EnvironmentId, request.ProjectId, request.ConfigId)
	if err != nil {
		return nil, apiError(err)
	}
	result := make([]*commonv1.ConfigEnum, 0, len(items))
	for index := range items {
		converted, err := converter.EnumToProto(&items[index])
		if err != nil {
			return nil, apiError(err)
		}
		result = append(result, converted)
	}
	return &adminv1.ListEnumReply{List: result}, nil
}

// DeleteEnum maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *emptypb.Empty on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *EnumService) DeleteEnum(ctx context.Context, request *adminv1.EnumIDRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	if err := s.enumLogic.Delete(ctx, actor, request.EnvironmentId, request.Id); err != nil {
		return nil, apiError(err)
	}
	return &emptypb.Empty{}, nil
}

func enumReply(item *model.ConfigEnum, err error) (*adminv1.EnumReply, error) {
	if err != nil {
		return nil, apiError(err)
	}
	converted, err := converter.EnumToProto(item)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.EnumReply{Enum: converted}, nil
}
