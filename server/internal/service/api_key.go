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

type apiKeyLogic interface {
	List(ctx context.Context, actor permission.Actor, environmentID, projectID int64) ([]model.ProjectAPIKey, error)
	Create(ctx context.Context, actor permission.Actor, environmentID, projectID int64, name string) (*logic.APIKeySecretResult, error)
	Rotate(ctx context.Context, actor permission.Actor, environmentID, projectID, keyID int64) (*logic.APIKeySecretResult, error)
	Revoke(ctx context.Context, actor permission.Actor, environmentID, projectID, keyID int64) error
}

type APIKeyService struct {
	apiKeyLogic apiKeyLogic
}

// NewAPIKeyService binds API handling to the APIKey business operations.
//
// Parameters: logicLayer implements authorization and transactional business operations.
// Returns: the configured APIKeyService.
// Throws: none.
func NewAPIKeyService(logicLayer *logic.APIKeyLogic) *APIKeyService {
	return &APIKeyService{apiKeyLogic: logicLayer}
}

var _ adminv1.ProjectApiKeyServiceHTTPServer = (*APIKeyService)(nil)

// ListProjectApiKeys returns scoped public credential metadata without secret digests.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
//
// The method spelling is fixed by the published protobuf HTTP server interface.
func (s *APIKeyService) ListProjectApiKeys(ctx context.Context, request *adminv1.ProjectApiKeyScopeRequest) (*adminv1.ListProjectApiKeysReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	items, err := s.apiKeyLogic.List(ctx, actor, request.EnvironmentId, request.ProjectId)
	if err != nil {
		return nil, apiError(err)
	}
	result := make([]*commonv1.ProjectApiKey, 0, len(items))
	for index := range items {
		result = append(result, converter.APIKeyToProto(&items[index]))
	}
	return &adminv1.ListProjectApiKeysReply{List: result}, nil
}

// CreateProjectApiKey returns a generated secret only in its successful creation response.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
//
// The method spelling is fixed by the published protobuf HTTP server interface.
func (s *APIKeyService) CreateProjectApiKey(ctx context.Context, request *adminv1.CreateProjectApiKeyRequest) (*adminv1.ProjectApiKeySecretReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	result, err := s.apiKeyLogic.Create(ctx, actor, request.EnvironmentId, request.ProjectId, request.Name)
	return secretReply(result, err)
}

// RotateProjectApiKey returns the replacement secret only after rotation succeeds.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
//
// The method spelling is fixed by the published protobuf HTTP server interface.
func (s *APIKeyService) RotateProjectApiKey(ctx context.Context, request *adminv1.ProjectApiKeyIDRequest) (*adminv1.ProjectApiKeySecretReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	result, err := s.apiKeyLogic.Rotate(ctx, actor, request.EnvironmentId, request.ProjectId, request.KeyId)
	return secretReply(result, err)
}

// RevokeProjectApiKey forwards revocation for the authenticated environment and project.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: an empty success response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
//
// The method spelling is fixed by the published protobuf HTTP server interface.
func (s *APIKeyService) RevokeProjectApiKey(ctx context.Context, request *adminv1.ProjectApiKeyIDRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	if err := s.apiKeyLogic.Revoke(ctx, actor, request.EnvironmentId, request.ProjectId, request.KeyId); err != nil {
		return nil, apiError(err)
	}
	return &emptypb.Empty{}, nil
}

func secretReply(result *logic.APIKeySecretResult, err error) (*adminv1.ProjectApiKeySecretReply, error) {
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ProjectApiKeySecretReply{
		ApiKey: converter.APIKeyToProto(result.Key),
		Secret: result.Secret,
	}, nil
}
