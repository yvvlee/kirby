package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type structureLogic interface {
	Create(ctx context.Context, param *entity.CreateStructureParam) (*commonv1.Structure, error)
	Update(ctx context.Context, param *entity.UpdateStructureParam) (*commonv1.Structure, error)
	List(ctx context.Context, param *entity.ListStructureParam) ([]*commonv1.Structure, error)
	Delete(ctx context.Context, actor permission.Actor, environmentID, structureID int64) error
}
type StructureService struct{ structureLogic structureLogic }

// NewStructureService creates StructureService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *StructureService.
// Throws: none.
func NewStructureService(logicLayer *logic.StructureLogic) *StructureService {
	return &StructureService{structureLogic: logicLayer}
}

var _ adminv1.StructureServiceHTTPServer = (*StructureService)(nil)

// CreateStructure maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.StructureReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *StructureService) CreateStructure(ctx context.Context, request *adminv1.CreateStructureRequest) (*adminv1.StructureReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.structureLogic.Create(ctx, &entity.CreateStructureParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ConfigID:      request.ConfigId,
		Key:           request.Key,
		Name:          request.Name,
		Description:   request.Description,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.StructureReply{Structure: item}, nil
}

// UpdateStructure maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.StructureReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *StructureService) UpdateStructure(ctx context.Context, request *adminv1.UpdateStructureRequest) (*adminv1.StructureReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.structureLogic.Update(ctx, &entity.UpdateStructureParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		StructureID:   request.Id,
		Key:           request.Key,
		Name:          request.Name,
		Description:   request.Description,
		Fields:        request.Fields,
		Version:       int64(request.Version),
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.StructureReply{Structure: item}, nil
}

// ListStructure maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ListStructureReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *StructureService) ListStructure(ctx context.Context, request *adminv1.ListStructureRequest) (*adminv1.ListStructureReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	items, err := s.structureLogic.List(ctx, &entity.ListStructureParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ProjectID:     request.ProjectId,
		ConfigID:      request.ConfigId,
		IgnoreID:      request.IgnoreDependencyId,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ListStructureReply{List: items}, nil
}

// DeleteStructure maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *emptypb.Empty on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *StructureService) DeleteStructure(ctx context.Context, request *adminv1.StructureIDRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	if err := s.structureLogic.Delete(ctx, actor, request.EnvironmentId, request.Id); err != nil {
		return nil, apiError(err)
	}
	return &emptypb.Empty{}, nil
}
