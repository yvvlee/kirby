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
	"github.com/yvvlee/kirby/server/internal/repository"
)

type roleLogic interface {
	List(ctx context.Context, _ permission.Actor) ([]repository.RoleWithPermissions, error)
	Create(ctx context.Context, actor permission.Actor, role *model.Role) (*repository.RoleWithPermissions, error)
	Update(ctx context.Context, actor permission.Actor, roleID int64, update repository.RoleUpdate) (*repository.RoleWithPermissions, error)
	Delete(ctx context.Context, actor permission.Actor, roleID int64) error
	ListPermissions(ctx context.Context, actor permission.Actor) ([]model.Permission, error)
	UpdatePermissions(ctx context.Context, actor permission.Actor, roleID int64, permissionIDs []int64) error
}

type RoleService struct{ roleLogic roleLogic }

// NewRoleService binds API handling to the Role business operations.
//
// Parameters: logicLayer implements authorization and transactional business operations.
// Returns: the configured RoleService.
// Throws: none.
func NewRoleService(logicLayer *logic.RoleLogic) *RoleService {
	return &RoleService{roleLogic: logicLayer}
}

var _ adminv1.RoleServiceHTTPServer = (*RoleService)(nil)

// ListRoles uses the shared role projection for the authenticated catalog.
//
// Parameters: ctx carries the authenticated transport; the request body is empty.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *RoleService) ListRoles(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListRolesReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.roleLogic.List(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.Role, 0, len(items))
	for index := range items {
		converted, err := converter.RoleToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListRolesReply{List: result}, nil
}

// CreateRole forwards custom-role creation and returns the persisted role projection.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *RoleService) CreateRole(ctx context.Context, request *adminv1.CreateRoleRequest) (*adminv1.RoleReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	created, err := s.roleLogic.Create(ctx, actor, &model.Role{
		Key:         request.Key,
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := converter.RoleToProto(created)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.RoleReply{Role: converted}, nil
}

// UpdateRole preserves the expected version when forwarding a role edit.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *RoleService) UpdateRole(ctx context.Context, request *adminv1.UpdateRoleRequest) (*adminv1.RoleReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	updated, err := s.roleLogic.Update(ctx, actor, request.RoleId, repository.RoleUpdate{
		Name:        request.Name,
		Description: request.Description,
		Version:     int64(request.Version),
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := converter.RoleToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.RoleReply{Role: converted}, nil
}

// DeleteRole forwards an authenticated deletion while preserving protected-role errors.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: an empty success response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *RoleService) DeleteRole(ctx context.Context, request *adminv1.RoleIDRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.roleLogic.Delete(ctx, actor, request.RoleId); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

// ListPermissions exposes the permission catalog through the shared public projection.
//
// Parameters: ctx carries the authenticated transport; the request body is empty.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *RoleService) ListPermissions(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListPermissionsReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.roleLogic.ListPermissions(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.Permission, 0, len(items))
	for index := range items {
		converted, err := converter.PermissionToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListPermissionsReply{List: result}, nil
}

// UpdateRolePermissions forwards the complete validated replacement permission set.
//
// Parameters: ctx carries the authenticated transport; request identifies the resource scope and requested fields.
// Returns: an empty success response, or a sanitized authorization, conflict, conversion, or persistence error.
// Throws: none; failures are returned as errors.
func (s *RoleService) UpdateRolePermissions(ctx context.Context, request *adminv1.UpdateRolePermissionsRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.roleLogic.UpdatePermissions(ctx, actor, request.RoleId, request.PermissionIds); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}
