package user

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	errorsv1 "github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	logic "github.com/yvvlee/kirby/server/internal/logic/user"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/safeint"
)

type Logic interface {
	List(context.Context, permission.Actor) ([]model.User, error)
	Create(context.Context, permission.Actor, string, string, string, bool) (*model.User, error)
	Update(context.Context, permission.Actor, int64, string, bool, int64) (*model.User, error)
	UpdatePassword(context.Context, permission.Actor, int64, string) error
	UpdateStatus(context.Context, permission.Actor, int64, bool, int64) (*model.User, error)
}

type Service struct{ logic Logic }

func New(logic *logic.Logic) (*Service, error) {
	if logic == nil {
		return nil, fmt.Errorf("user service logic is nil")
	}
	return &Service{logic: logic}, nil
}

var _ adminv1.UserServiceHTTPServer = (*Service)(nil)

func (s *Service) ListUsers(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListUsersReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.logic.List(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.User, 0, len(items))
	for index := range items {
		converted, err := userToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListUsersReply{List: result}, nil
}

func (s *Service) CreateUser(ctx context.Context, request *adminv1.CreateUserRequest) (*adminv1.UserReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	created, err := s.logic.Create(ctx, actor, request.Username, request.DisplayName, request.Password, request.IsSystemAdmin)
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := userToProto(created)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.UserReply{User: converted}, nil
}

func (s *Service) UpdateUser(ctx context.Context, request *adminv1.UpdateUserRequest) (*adminv1.UserReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	updated, err := s.logic.Update(ctx, actor, request.UserId, request.DisplayName, request.IsSystemAdmin, int64(request.Version))
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := userToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.UserReply{User: converted}, nil
}

func (s *Service) UpdateUserPassword(ctx context.Context, request *adminv1.UpdateUserPasswordRequest) (*emptypb.Empty, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.logic.UpdatePassword(ctx, actor, request.UserId, request.Password); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) UpdateUserStatus(ctx context.Context, request *adminv1.UpdateUserStatusRequest) (*adminv1.UserReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	updated, err := s.logic.UpdateStatus(ctx, actor, request.UserId, request.Enabled, int64(request.Version))
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := userToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.UserReply{User: converted}, nil
}

func userToProto(item *model.User) (*commonv1.User, error) {
	if item == nil || item.ID <= 0 {
		return nil, fmt.Errorf("invalid user record")
	}
	version, err := safeint.Uint32FromInt64(item.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid user record")
	}
	return &commonv1.User{
		Id: item.ID, Username: item.Username, DisplayName: item.DisplayName, Enabled: item.Enabled, IsSystemAdmin: item.IsSystemAdmin,
		CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt), Version: version,
	}, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
