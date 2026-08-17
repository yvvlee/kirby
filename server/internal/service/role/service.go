package role

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	errorsv1 "github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	logic "github.com/yvvlee/kirby/server/internal/logic/role"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type Logic interface {
	List(context.Context, permission.Actor) ([]repository.RoleWithPermissions, error)
	Create(context.Context, permission.Actor, *model.Role) (*repository.RoleWithPermissions, error)
	Update(context.Context, permission.Actor, int64, repository.RoleUpdate) (*repository.RoleWithPermissions, error)
	Delete(context.Context, permission.Actor, int64) error
	ListPermissions(context.Context, permission.Actor) ([]model.Permission, error)
	UpdatePermissions(context.Context, permission.Actor, int64, []int64) error
}

type Service struct{ logic Logic }

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("role service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.RoleServiceHTTPServer = (*Service)(nil)

func (s *Service) ListRoles(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListRolesReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.logic.List(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.Role, 0, len(items))
	for index := range items {
		converted, err := roleToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListRolesReply{List: result}, nil
}

func (s *Service) CreateRole(ctx context.Context, request *adminv1.CreateRoleRequest) (*adminv1.RoleReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	created, err := s.logic.Create(ctx, actor, &model.Role{Key: request.Key, Name: request.Name, Description: request.Description})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := roleToProto(created)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.RoleReply{Role: converted}, nil
}

func (s *Service) UpdateRole(ctx context.Context, request *adminv1.UpdateRoleRequest) (*adminv1.RoleReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	updated, err := s.logic.Update(ctx, actor, request.RoleId, repository.RoleUpdate{
		Name: request.Name, Description: request.Description, Version: int64(request.Version),
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := roleToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.RoleReply{Role: converted}, nil
}

func (s *Service) DeleteRole(ctx context.Context, request *adminv1.RoleIDRequest) (*emptypb.Empty, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.logic.Delete(ctx, actor, request.RoleId); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) ListPermissions(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListPermissionsReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.logic.ListPermissions(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.Permission, 0, len(items))
	for index := range items {
		converted, err := permissionToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListPermissionsReply{List: result}, nil
}

func (s *Service) UpdateRolePermissions(ctx context.Context, request *adminv1.UpdateRolePermissionsRequest) (*emptypb.Empty, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.logic.UpdatePermissions(ctx, actor, request.RoleId, request.PermissionIds); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func roleToProto(item *repository.RoleWithPermissions) (*commonv1.Role, error) {
	if item == nil || item.Role.ID <= 0 || item.Role.Version < 0 || uint64(item.Role.Version) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("invalid role record")
	}
	permissions := make([]*commonv1.Permission, 0, len(item.Permissions))
	for index := range item.Permissions {
		converted, err := permissionToProto(&item.Permissions[index])
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, converted)
	}
	return &commonv1.Role{
		Id: item.Role.ID, Key: item.Role.Key, Name: item.Role.Name, Description: item.Role.Description, Builtin: item.Role.Builtin,
		Permissions: permissions, CreatedAt: formatTime(item.Role.CreatedAt), UpdatedAt: formatTime(item.Role.UpdatedAt), Version: uint32(item.Role.Version),
	}, nil
}

func permissionToProto(item *model.Permission) (*commonv1.Permission, error) {
	if item == nil || item.ID <= 0 {
		return nil, fmt.Errorf("invalid permission record")
	}
	return &commonv1.Permission{Id: item.ID, Key: item.Key, Name: item.Name, Description: item.Description}, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
