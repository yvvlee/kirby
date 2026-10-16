package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type userLogic interface {
	List(ctx context.Context, actor permission.Actor) ([]model.User, error)
	Create(ctx context.Context, actor permission.Actor, input logic.UserCreate) (*model.User, error)
	Update(ctx context.Context, actor permission.Actor, input logic.UserUpdate) (*model.User, error)
	UpdatePassword(ctx context.Context, actor permission.Actor, userID int64, plainPassword string) error
	UpdateStatus(ctx context.Context, actor permission.Actor, userID int64, enabled bool, version int64) (*model.User, error)
}

type UserService struct{ userLogic userLogic }

// NewUserService binds API handling to the User business operations.
//
// Parameters: logicLayer implements authorization and transactional business operations.
// Returns: the configured UserService.
// Throws: none.
func NewUserService(logicLayer *logic.UserLogic) *UserService {
	return &UserService{userLogic: logicLayer}
}

var _ adminv1.UserServiceHTTPServer = (*UserService)(nil)

// ListUsers requires an authenticated actor before returning public account metadata.
//
// Parameters: ctx carries the authenticated transport; the request body is empty.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *UserService) ListUsers(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListUsersReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.userLogic.List(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.User, 0, len(items))
	for index := range items {
		converted, err := converter.UserToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListUsersReply{List: result}, nil
}

// CreateUser converts account-creation input into a password-safe business request.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *UserService) CreateUser(ctx context.Context, request *adminv1.CreateUserRequest) (*adminv1.UserReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	created, err := s.userLogic.Create(ctx, actor, logic.UserCreate{
		Username:      request.Username,
		DisplayName:   request.DisplayName,
		PlainPassword: request.Password,
		SystemAdmin:   request.IsSystemAdmin,
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := converter.UserToProto(created)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.UserReply{User: converted}, nil
}

// UpdateUser preserves the expected version when forwarding an account edit.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *UserService) UpdateUser(ctx context.Context, request *adminv1.UpdateUserRequest) (*adminv1.UserReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	updated, err := s.userLogic.Update(ctx, actor, logic.UserUpdate{
		UserID:      request.UserId,
		DisplayName: request.DisplayName,
		SystemAdmin: request.IsSystemAdmin,
		Version:     int64(request.Version),
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := converter.UserToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.UserReply{User: converted}, nil
}

// UpdateUserPassword forwards password changes without returning password material.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: an empty success response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *UserService) UpdateUserPassword(ctx context.Context, request *adminv1.UpdateUserPasswordRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.userLogic.UpdatePassword(ctx, actor, request.UserId, request.Password); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateUserStatus preserves the expected version when changing account availability.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *UserService) UpdateUserStatus(ctx context.Context, request *adminv1.UpdateUserStatusRequest) (*adminv1.UserReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	updated, err := s.userLogic.UpdateStatus(ctx, actor, request.UserId, request.Enabled, int64(request.Version))
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := converter.UserToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.UserReply{User: converted}, nil
}
