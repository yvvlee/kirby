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
	"github.com/yvvlee/kirby/server/internal/repository"
)

type configLogic interface {
	Create(ctx context.Context, actor permission.Actor, environmentID int64, input logic.ConfigCreateInput) (*model.Config, error)
	Update(ctx context.Context, actor permission.Actor, environmentID int64, input logic.ConfigUpdateInput) (*model.Config, error)
	UpdateValue(ctx context.Context, actor permission.Actor, environmentID int64, input logic.ConfigValueInput) (*model.Config, error)
	List(ctx context.Context, actor permission.Actor, environmentID int64, filter repository.ConfigFilter) ([]model.Config, map[int64]bool, error)
	Delete(ctx context.Context, actor permission.Actor, environmentID, configID int64) error
	Detail(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Config, *commonv1.TreeNode, error)
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
	item, err := s.configLogic.Create(ctx, actor, request.EnvironmentId, logic.ConfigCreateInput{
		ProjectID:   request.ProjectId,
		Key:         request.Key,
		Description: request.Description,
	})
	return configReply(item, err)
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
	item, err := s.configLogic.Update(ctx, actor, request.EnvironmentId, logic.ConfigUpdateInput{
		ConfigID:    request.Id,
		Description: request.Description,
		FieldType:   request.Type,
		IsArray:     request.IsArray,
		Version:     int64(request.Version),
	})
	return configReply(item, err)
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
	item, err := s.configLogic.UpdateValue(ctx, actor, request.EnvironmentId, logic.ConfigValueInput{
		ConfigID: request.Id,
		Value:    request.Value,
		Version:  int64(request.Version),
	})
	return configReply(item, err)
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
	items, released, err := s.configLogic.List(ctx, actor, request.EnvironmentId, repository.ConfigFilter{
		ProjectID:  request.ProjectId,
		ProjectKey: request.ProjectKey,
		Key:        request.Key,
	})
	if err != nil {
		return nil, apiError(err)
	}
	result := make([]*adminv1.ListConfigItem, 0, len(items))
	for index := range items {
		converted, err := converter.ConfigToProto(&items[index])
		if err != nil {
			return nil, apiError(err)
		}
		result = append(result, &adminv1.ListConfigItem{
			Config:     converted,
			IsReleased: released[items[index].ID],
		})
	}
	return &adminv1.ListConfigReply{List: result}, nil
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
	converted, err := converter.ConfigToProto(item)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ConfigDetailReply{
		Config: converted,
		Tree:   tree,
	}, nil
}

func configReply(item *model.Config, err error) (*adminv1.ConfigReply, error) {
	if err != nil {
		return nil, apiError(err)
	}
	converted, err := converter.ConfigToProto(item)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ConfigReply{Config: converted}, nil
}
