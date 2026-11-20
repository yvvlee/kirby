package service

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/auth/passwordpolicy"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type userLogic interface {
	List(ctx context.Context, actor permission.Actor) ([]*commonv1.User, error)
	Create(ctx context.Context, param *entity.UserCreateParam) (*commonv1.User, error)
	Update(ctx context.Context, param *entity.UserUpdateParam) (*commonv1.User, error)
	UpdatePassword(ctx context.Context, actor permission.Actor, userID int64, plainPassword string) error
	UpdateStatus(ctx context.Context, param *entity.UpdateUserStatusParam) (*commonv1.User, error)
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
	return &adminv1.ListUsersReply{List: items}, nil
}

// CreateUser converts account-creation input into a password-safe business request.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *UserService) CreateUser(ctx context.Context, request *adminv1.CreateUserRequest) (*adminv1.UserReply, error) {
	if err := passwordpolicy.Validate(request.Password); err != nil {
		return nil, errorsv1.ErrorBadRequest("%s", err.Error())
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	created, err := s.userLogic.Create(ctx, &entity.UserCreateParam{
		Actor:         actor,
		Username:      strings.ToLower(strings.TrimSpace(request.Username)),
		Email:         strings.ToLower(strings.TrimSpace(request.Email)),
		Nickname:      request.Nickname,
		PlainPassword: request.Password,
		SystemAdmin:   request.IsSystemAdmin,
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.UserReply{User: created}, nil
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
	updated, err := s.userLogic.Update(ctx, &entity.UserUpdateParam{
		Actor:       actor,
		UserID:      request.UserId,
		Nickname:    request.Nickname,
		SystemAdmin: request.IsSystemAdmin,
		Version:     int64(request.Version),
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.UserReply{User: updated}, nil
}

// UpdateUserPassword forwards password changes without returning password material.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: an empty success response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *UserService) UpdateUserPassword(ctx context.Context, request *adminv1.UpdateUserPasswordRequest) (*emptypb.Empty, error) {
	if err := passwordpolicy.Validate(request.Password); err != nil {
		return nil, errorsv1.ErrorBadRequest("%s", err.Error())
	}
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
	updated, err := s.userLogic.UpdateStatus(ctx, &entity.UpdateUserStatusParam{
		Actor:   actor,
		UserID:  request.UserId,
		Enabled: request.Enabled,
		Version: int64(request.Version),
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.UserReply{User: updated}, nil
}
