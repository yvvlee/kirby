package logic

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type ProjectMemberLogic struct {
	members      repository.ProjectMemberRepository
	users        repository.UserRepository
	transactions TransactionRunner
	projects     repository.ProjectRepository
}

// NewProjectMemberLogic maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func NewProjectMemberLogic(members repository.ProjectMemberRepository, users repository.UserRepository, transactions TransactionRunner, projects repository.ProjectRepository) *ProjectMemberLogic {
	return &ProjectMemberLogic{
		members:      members,
		users:        users,
		transactions: transactions,
		projects:     projects,
	}
}

func (s *ProjectMemberLogic) actorRole(ctx context.Context, projectID int64) (permission.ProjectRole, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return "", err
	}
	member, err := s.members.Get(ctx, projectID, actor.UserID)
	if err != nil {
		return "", err
	}
	return permission.ProjectRole(member.Role), nil
}

// ListProjectMembers maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberLogic) ListProjectMembers(ctx context.Context, req *adminv1.ProjectScopeRequest) (*adminv1.ListProjectMembersReply, error) {
	if _, err := s.actorRole(ctx, req.ProjectId); err != nil {
		return nil, err
	}
	items, err := s.members.List(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListProjectMembersReply{List: make([]*adminv1.ProjectMemberReply, 0, len(items))}
	for index := range items {
		user, err := s.users.GetByID(ctx, items[index].UserID)
		if err != nil {
			return nil, err
		}
		converted, err := converter.UserToProto(user)
		if err != nil {
			return nil, err
		}
		reply.List = append(reply.List, &adminv1.ProjectMemberReply{
			User: converted,
			Role: items[index].Role,
		})
	}
	return reply, nil
}

func (s *ProjectMemberLogic) mutateAddProjectMember(ctx context.Context, req *adminv1.AddProjectMemberRequest) (*adminv1.ProjectMemberReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, err
	}
	role, err := s.actorRole(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	if !permission.CanGrant(role, permission.ProjectRole(req.Role)) {
		return nil, permission.ErrForbidden
	}
	user, err := s.users.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	if !user.Enabled {
		return nil, permission.ErrForbidden
	}
	err = s.members.Add(ctx, &model.ProjectMember{
		ProjectID: req.ProjectId,
		UserID:    req.UserId,
		Role:      req.Role,
		CreatedBy: actor.UserID,
	})
	if err != nil {
		return nil, err
	}
	converted, err := converter.UserToProto(user)
	if err != nil {
		return nil, err
	}
	return &adminv1.ProjectMemberReply{
		User: converted,
		Role: req.Role,
	}, nil
}

func (s *ProjectMemberLogic) mutateUpdateProjectMemberRole(ctx context.Context, req *adminv1.UpdateProjectMemberRoleRequest) (*adminv1.ProjectMemberReply, error) {
	role, err := s.actorRole(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	if !permission.CanGrant(role, permission.ProjectRole(req.Role)) {
		return nil, permission.ErrForbidden
	}
	if req.Role == string(permission.ProjectRoleOwner) && role != permission.ProjectRoleOwner {
		return nil, permission.ErrForbidden
	}
	target, err := s.members.Get(ctx, req.ProjectId, req.UserId)
	if err != nil {
		return nil, err
	}
	if !permission.CanGrant(role, permission.ProjectRole(target.Role)) {
		return nil, permission.ErrForbidden
	}
	if target.Role == "OWNER" && req.Role != "OWNER" {
		owners, err := s.members.CountOwners(ctx, req.ProjectId)
		if err != nil {
			return nil, err
		}
		if owners <= 1 {
			return nil, permission.ErrForbidden
		}
	}
	if err := s.members.UpdateRole(ctx, req.ProjectId, req.UserId, req.Role); err != nil {
		return nil, err
	}
	user, err := s.users.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	converted, err := converter.UserToProto(user)
	if err != nil {
		return nil, err
	}
	return &adminv1.ProjectMemberReply{
		User: converted,
		Role: req.Role,
	}, nil
}

func (s *ProjectMemberLogic) mutateRemoveProjectMember(ctx context.Context, req *adminv1.ProjectMemberTargetRequest) (*emptypb.Empty, error) {
	role, err := s.actorRole(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	target, err := s.members.Get(ctx, req.ProjectId, req.UserId)
	if err != nil {
		return nil, err
	}
	if !permission.CanGrant(role, permission.ProjectRole(target.Role)) {
		return nil, permission.ErrForbidden
	}
	if target.Role == string(permission.ProjectRoleOwner) {
		owners, err := s.members.CountOwners(ctx, req.ProjectId)
		if err != nil {
			return nil, err
		}
		if owners <= 1 {
			return nil, permission.ErrForbidden
		}
	}
	if err := s.members.Remove(ctx, req.ProjectId, req.UserId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// AddProjectMember maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberLogic) AddProjectMember(ctx context.Context, req *adminv1.AddProjectMemberRequest) (*adminv1.ProjectMemberReply, error) {
	var result *adminv1.ProjectMemberReply
	err := s.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := s.projects.LockByID(txctx, 0, req.ProjectId); err != nil {
			return err
		}
		var err error
		result, err = s.mutateAddProjectMember(txctx, req)
		return err
	})
	return result, err
}

// UpdateProjectMemberRole maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberLogic) UpdateProjectMemberRole(ctx context.Context, req *adminv1.UpdateProjectMemberRoleRequest) (*adminv1.ProjectMemberReply, error) {
	var result *adminv1.ProjectMemberReply
	err := s.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := s.projects.LockByID(txctx, 0, req.ProjectId); err != nil {
			return err
		}
		var err error
		result, err = s.mutateUpdateProjectMemberRole(txctx, req)
		return err
	})
	return result, err
}

// RemoveProjectMember maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberLogic) RemoveProjectMember(ctx context.Context, req *adminv1.ProjectMemberTargetRequest) (*emptypb.Empty, error) {
	var result *emptypb.Empty
	err := s.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := s.projects.LockByID(txctx, 0, req.ProjectId); err != nil {
			return err
		}
		var err error
		result, err = s.mutateRemoveProjectMember(txctx, req)
		return err
	})
	return result, err
}

// SearchProjectUsers maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberLogic) SearchProjectUsers(ctx context.Context, req *adminv1.SearchProjectUsersRequest) (*adminv1.SearchProjectUsersReply, error) {
	role, err := s.actorRole(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	if role != permission.ProjectRoleOwner && role != permission.ProjectRoleMaintainer {
		return nil, permission.ErrForbidden
	}
	users, err := s.users.ListManagedUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := &adminv1.SearchProjectUsersReply{}
	for index := range users {
		user := &users[index]
		if !user.Enabled || !strings.Contains(strings.ToLower(user.Username+" "+user.Email+" "+user.Nickname), strings.ToLower(req.Keyword)) {
			continue
		}
		converted, err := converter.UserToProto(user)
		if err != nil {
			return nil, err
		}
		result.List = append(result.List, converted)
	}
	return result, nil
}
