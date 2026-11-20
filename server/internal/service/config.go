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

type configLogic interface {
	Create(ctx context.Context, param *entity.CreateConfigParam) (*commonv1.Config, error)
	Update(ctx context.Context, param *entity.UpdateConfigParam) (*commonv1.Config, error)
	UpdateValue(ctx context.Context, param *entity.UpdateConfigValueParam) (*commonv1.Config, error)
	List(ctx context.Context, param *entity.ListConfigParam) ([]*adminv1.ListConfigItem, error)
	Delete(ctx context.Context, actor permission.Actor, environmentID, configID int64) error
	Detail(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*commonv1.Config, *commonv1.TreeNode, error)
}

type ConfigService struct{ configLogic configLogic }

// NewConfigService creates ConfigService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *ConfigService.
// Throws: none.
func NewConfigService(logicLayer *logic.ConfigLogic) *ConfigService {
	return &ConfigService{configLogic: logicLayer}
}

var _ adminv1.ConfigServiceHTTPServer = (*ConfigService)(nil)

// CreateConfig maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ConfigReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ConfigService) CreateConfig(ctx context.Context, request *adminv1.CreateConfigRequest) (*adminv1.ConfigReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.configLogic.Create(ctx, &entity.CreateConfigParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ProjectID:     request.ProjectId,
		Key:           request.Key,
		Description:   request.Description,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ConfigReply{Config: item}, nil
}

// UpdateConfig maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ConfigReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ConfigService) UpdateConfig(ctx context.Context, request *adminv1.UpdateConfigRequest) (*adminv1.ConfigReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.configLogic.Update(ctx, &entity.UpdateConfigParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ConfigID:      request.Id,
		Description:   request.Description,
		FieldType:     request.Type,
		IsArray:       request.IsArray,
		Version:       int64(request.Version),
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ConfigReply{Config: item}, nil
}

// UpdateConfigValue maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ConfigReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ConfigService) UpdateConfigValue(ctx context.Context, request *adminv1.UpdateConfigValueRequest) (*adminv1.ConfigReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.configLogic.UpdateValue(ctx, &entity.UpdateConfigValueParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ConfigID:      request.Id,
		Value:         request.Value,
		Version:       int64(request.Version),
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ConfigReply{Config: item}, nil
}

// ListConfig maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ListConfigReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ConfigService) ListConfig(ctx context.Context, request *adminv1.ListConfigRequest) (*adminv1.ListConfigReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	items, err := s.configLogic.List(ctx, &entity.ListConfigParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		Filter: repository.ConfigFilter{
			ProjectID:  request.ProjectId,
			ProjectKey: request.ProjectKey,
			Key:        request.Key,
		},
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ListConfigReply{List: items}, nil
}

// DeleteConfig maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *emptypb.Empty on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ConfigService) DeleteConfig(ctx context.Context, request *adminv1.ConfigIDRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	if err := s.configLogic.Delete(ctx, actor, request.EnvironmentId, request.Id); err != nil {
		return nil, apiError(err)
	}
	return &emptypb.Empty{}, nil
}

// ConfigDetail maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ConfigDetailReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ConfigService) ConfigDetail(ctx context.Context, request *adminv1.ConfigIDRequest) (*adminv1.ConfigDetailReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, tree, err := s.configLogic.Detail(ctx, actor, request.EnvironmentId, request.Id)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ConfigDetailReply{
		Config: item,
		Tree:   tree,
	}, nil
}
