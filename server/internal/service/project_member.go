package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/logic"
)

type ProjectMemberService struct{ logic *logic.ProjectMemberLogic }

// NewProjectMemberService maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func NewProjectMemberService(l *logic.ProjectMemberLogic) *ProjectMemberService {
	return &ProjectMemberService{logic: l}
}

// ListProjectMembers maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberService) ListProjectMembers(ctx context.Context, req *adminv1.ProjectScopeRequest) (*adminv1.ListProjectMembersReply, error) {
	result, err := s.logic.ListProjectMembers(ctx, req)
	if err != nil {
		return nil, apiError(err)
	}
	return result, nil
}

// AddProjectMember maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberService) AddProjectMember(ctx context.Context, req *adminv1.AddProjectMemberRequest) (*adminv1.ProjectMemberReply, error) {
	result, err := s.logic.AddProjectMember(ctx, req)
	if err != nil {
		return nil, apiError(err)
	}
	return result, nil
}

// UpdateProjectMemberRole maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberService) UpdateProjectMemberRole(ctx context.Context, req *adminv1.UpdateProjectMemberRoleRequest) (*adminv1.ProjectMemberReply, error) {
	result, err := s.logic.UpdateProjectMemberRole(ctx, req)
	if err != nil {
		return nil, apiError(err)
	}
	return result, nil
}

// RemoveProjectMember maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberService) RemoveProjectMember(ctx context.Context, req *adminv1.ProjectMemberTargetRequest) (*emptypb.Empty, error) {
	result, err := s.logic.RemoveProjectMember(ctx, req)
	if err != nil {
		return nil, apiError(err)
	}
	return result, nil
}

// SearchProjectUsers maintains project membership boundaries.
//
// Parameters: dependencies or request context and project-scoped input.
// Returns: the requested result or an authorization or persistence error.
// Throws: none; errors are returned.
func (s *ProjectMemberService) SearchProjectUsers(ctx context.Context, req *adminv1.SearchProjectUsersRequest) (*adminv1.SearchProjectUsersReply, error) {
	result, err := s.logic.SearchProjectUsers(ctx, req)
	if err != nil {
		return nil, apiError(err)
	}
	return result, nil
}
