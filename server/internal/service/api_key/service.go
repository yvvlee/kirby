package api_key

import (
	"context"
	"fmt"
	"strconv"
	"time"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/api_key"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/timeutil"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Logic interface {
	List(context.Context, permission.Actor, int64, int64) ([]model.ProjectAPIKey, error)
	Create(context.Context, permission.Actor, int64, int64, string) (*logic.SecretResult, error)
	Rotate(context.Context, permission.Actor, int64, int64, int64) (*logic.SecretResult, error)
	Revoke(context.Context, permission.Actor, int64, int64, int64) error
}

type Service struct {
	logic Logic
}

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("project API key service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.ProjectApiKeyServiceHTTPServer = (*Service)(nil)

func (s *Service) ListProjectApiKeys(ctx context.Context, request *adminv1.ProjectApiKeyScopeRequest) (*adminv1.ListProjectApiKeysReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	items, err := s.logic.List(ctx, actor, request.EnvironmentId, request.ProjectId)
	if err != nil {
		return nil, entity.APIError(err)
	}
	result := make([]*commonv1.ProjectApiKey, 0, len(items))
	for index := range items {
		result = append(result, toProto(&items[index]))
	}
	return &adminv1.ListProjectApiKeysReply{List: result}, nil
}

func (s *Service) CreateProjectApiKey(ctx context.Context, request *adminv1.CreateProjectApiKeyRequest) (*adminv1.ProjectApiKeySecretReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	result, err := s.logic.Create(ctx, actor, request.EnvironmentId, request.ProjectId, request.Name)
	return secretReply(result, err)
}

func (s *Service) RotateProjectApiKey(ctx context.Context, request *adminv1.ProjectApiKeyIDRequest) (*adminv1.ProjectApiKeySecretReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	result, err := s.logic.Rotate(ctx, actor, request.EnvironmentId, request.ProjectId, request.KeyId)
	return secretReply(result, err)
}

func (s *Service) RevokeProjectApiKey(ctx context.Context, request *adminv1.ProjectApiKeyIDRequest) (*emptypb.Empty, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	if err := s.logic.Revoke(ctx, actor, request.EnvironmentId, request.ProjectId, request.KeyId); err != nil {
		return nil, entity.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func secretReply(result *logic.SecretResult, err error) (*adminv1.ProjectApiKeySecretReply, error) {
	if err != nil {
		return nil, entity.APIError(err)
	}
	if result == nil || result.Key == nil || result.Secret == "" {
		return nil, entity.APIError(fmt.Errorf("project API key logic returned an incomplete secret"))
	}
	return &adminv1.ProjectApiKeySecretReply{ApiKey: toProto(result.Key), Secret: result.Secret}, nil
}

func toProto(item *model.ProjectAPIKey) *commonv1.ProjectApiKey {
	if item == nil {
		return nil
	}
	return &commonv1.ProjectApiKey{
		Id: item.ID, PublicId: item.PublicID, Name: item.Name,
		CreatedBy: strconv.FormatInt(item.CreatedBy, 10), CreatedAt: timeutil.FormatRFC3339(item.CreatedAt),
		LastUsedAt: optionalTime(item.LastUsedAt), RevokedAt: optionalTime(item.RevokedAt), SecretSuffix: item.SecretSuffix,
	}
}

func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := timeutil.FormatRFC3339(*value)
	return &formatted
}

func badRequest() error {
	return entity.APIError(entity.Invalid("invalid request"))
}
