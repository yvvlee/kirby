package environment

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	logic "github.com/yvvlee/kirby/server/internal/logic/environment"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/safeint"
)

type Logic interface {
	List(context.Context, permission.Actor) ([]model.Environment, error)
	Create(context.Context, permission.Actor, *model.Environment) (*model.Environment, error)
	Update(context.Context, permission.Actor, int64, repository.EnvironmentUpdate) (*model.Environment, error)
	Permissions(context.Context, permission.Actor, int64) ([]string, error)
	ListMembers(context.Context, permission.Actor, int64) ([]repository.EnvironmentMemberRecord, error)
	UpdateMemberRoles(context.Context, permission.Actor, int64, int64, []int64) error
}

type Service struct{ logic Logic }

func New(logic *logic.Logic) (*Service, error) {
	if logic == nil {
		return nil, fmt.Errorf("environment service logic is nil")
	}
	return &Service{logic: logic}, nil
}

var _ adminv1.EnvironmentServiceHTTPServer = (*Service)(nil)

func (s *Service) ListEnvironments(ctx context.Context, request *adminv1.ListEnvironmentsRequest) (*adminv1.ListEnvironmentsReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	items, err := s.logic.List(ctx, actor)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if request != nil && request.ProjectId != nil {
		filtered := items[:0]
		for _, item := range items {
			if item.ProjectID == *request.ProjectId {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	result := make([]*commonv1.Environment, 0, len(items))
	for index := range items {
		converted, err := environmentToProto(&items[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, converted)
	}
	return &adminv1.ListEnvironmentsReply{List: result}, nil
}

func (s *Service) CreateEnvironment(ctx context.Context, request *adminv1.CreateEnvironmentRequest) (*adminv1.EnvironmentReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	created, err := s.logic.Create(ctx, actor, &model.Environment{ProjectID: request.ProjectId, Key: request.Key, Name: request.Name, Description: request.Description})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := environmentToProto(created)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.EnvironmentReply{Environment: converted}, nil
}

func (s *Service) UpdateEnvironment(ctx context.Context, request *adminv1.UpdateEnvironmentRequest) (*adminv1.EnvironmentReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if verifier, ok := s.logic.(interface {
		VerifyProject(context.Context, int64, int64) error
	}); ok {
		if err := verifier.VerifyProject(ctx, request.EnvironmentId, request.ProjectId); err != nil {
			return nil, permission.APIError(err)
		}
	}
	updated, err := s.logic.Update(ctx, actor, request.EnvironmentId, repository.EnvironmentUpdate{
		Name: request.Name, Description: request.Description, Enabled: request.Enabled, Version: int64(request.Version),
	})
	if err != nil {
		return nil, permission.APIError(err)
	}
	converted, err := environmentToProto(updated)
	if err != nil {
		return nil, errorsv1.ErrorInternal("operation failed")
	}
	return &adminv1.EnvironmentReply{Environment: converted}, nil
}

func (s *Service) MyPermissions(ctx context.Context, request *adminv1.EnvironmentIDRequest) (*adminv1.MyPermissionsReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	keys, err := s.logic.Permissions(ctx, actor, request.EnvironmentId)
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.MyPermissionsReply{Permissions: keys}, nil
}

func (s *Service) ListEnvironmentUsers(ctx context.Context, request *adminv1.EnvironmentIDRequest) (*adminv1.ListEnvironmentUsersReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	members, err := s.logic.ListMembers(ctx, actor, request.EnvironmentId)
	if err != nil {
		return nil, permission.APIError(err)
	}
	result := make([]*commonv1.EnvironmentMember, 0, len(members))
	for index := range members {
		member, err := memberToProto(&members[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("operation failed")
		}
		result = append(result, member)
	}
	return &adminv1.ListEnvironmentUsersReply{List: result}, nil
}

func (s *Service) UpdateEnvironmentUserRoles(ctx context.Context, request *adminv1.UpdateEnvironmentUserRolesRequest) (*emptypb.Empty, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, errorsv1.ErrorBadRequest("invalid request")
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	if err := s.logic.UpdateMemberRoles(ctx, actor, request.EnvironmentId, request.UserId, request.RoleIds); err != nil {
		return nil, permission.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func environmentToProto(item *model.Environment) (*commonv1.Environment, error) {
	if item == nil || item.ID <= 0 {
		return nil, fmt.Errorf("invalid environment record")
	}
	version, err := safeint.Uint32FromInt64(item.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid environment record")
	}
	return &commonv1.Environment{
		Id: item.ID, ProjectId: item.ProjectID, Key: item.Key, Name: item.Name, Description: item.Description, Enabled: item.Enabled,
		CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt), Version: version,
	}, nil
}

func memberToProto(item *repository.EnvironmentMemberRecord) (*commonv1.EnvironmentMember, error) {
	if item == nil {
		return nil, fmt.Errorf("invalid environment member")
	}
	user, err := userToProto(&item.User)
	if err != nil {
		return nil, err
	}
	roles := make([]*commonv1.Role, 0, len(item.Roles))
	for index := range item.Roles {
		converted, err := roleToProto(&item.Roles[index])
		if err != nil {
			return nil, err
		}
		roles = append(roles, converted)
	}
	return &commonv1.EnvironmentMember{User: user, Roles: roles}, nil
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

func roleToProto(item *repository.RoleWithPermissions) (*commonv1.Role, error) {
	if item == nil || item.Role.ID <= 0 {
		return nil, fmt.Errorf("invalid role record")
	}
	version, err := safeint.Uint32FromInt64(item.Role.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid role record")
	}
	permissions := make([]*commonv1.Permission, 0, len(item.Permissions))
	for _, value := range item.Permissions {
		permissions = append(permissions, &commonv1.Permission{Id: value.ID, Key: value.Key, Name: value.Name, Description: value.Description})
	}
	return &commonv1.Role{
		Id: item.Role.ID, Key: item.Role.Key, Name: item.Role.Name, Description: item.Role.Description, Builtin: item.Role.Builtin,
		Permissions: permissions, CreatedAt: formatTime(item.Role.CreatedAt), UpdatedAt: formatTime(item.Role.UpdatedAt), Version: version,
	}, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
