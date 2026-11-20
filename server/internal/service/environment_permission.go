package service

import (
	"context"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type EnvironmentPermissionService struct {
	logic *logic.EnvironmentPermissionLogic
}

// NewEnvironmentPermissionService enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func NewEnvironmentPermissionService(logicLayer *logic.EnvironmentPermissionLogic) *EnvironmentPermissionService {
	return &EnvironmentPermissionService{logic: logicLayer}
}

var _ adminv1.EnvironmentPermissionServiceHTTPServer = (*EnvironmentPermissionService)(nil)

// GetEnvironmentPermissionMatrix enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func (s *EnvironmentPermissionService) GetEnvironmentPermissionMatrix(ctx context.Context, req *adminv1.EnvironmentPermissionScopeRequest) (*adminv1.EnvironmentPermissionMatrixReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	roles, err := s.logic.Matrix(ctx, actor, req.EnvironmentId)
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.EnvironmentPermissionMatrixReply{Roles: roles}, nil
}

// UpdateEnvironmentPermissionMatrix enforces environment permission boundaries.
// Parameters: context and the explicitly scoped input or dependencies.
// Returns: the requested result or a persistence or authorization error.
// Throws: none; failures are returned.
func (s *EnvironmentPermissionService) UpdateEnvironmentPermissionMatrix(ctx context.Context, req *adminv1.UpdateEnvironmentPermissionMatrixRequest) (*adminv1.EnvironmentPermissionMatrixReply, error) {
	if err := validateMatrix(req.Roles); err != nil {
		return nil, errorsv1.ErrorInvalidParam("%s", err)
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, permission.APIError(err)
	}
	roles, err := s.logic.Update(ctx, actor, req.EnvironmentId, req.Roles)
	if err != nil {
		return nil, permission.APIError(err)
	}
	return &adminv1.EnvironmentPermissionMatrixReply{Roles: roles}, nil
}

func validateMatrix(roles []*adminv1.EnvironmentPermissionRole) error {
	seen := map[string]bool{}
	for _, role := range roles {
		if role == nil {
			return fmt.Errorf("missing role")
		}
		if seen[role.Role] {
			return fmt.Errorf("duplicate role")
		}
		seen[role.Role] = true
		if err := validateRolePermissions(role); err != nil {
			return err
		}
	}
	return nil
}

func validateRolePermissions(role *adminv1.EnvironmentPermissionRole) error {
	switch role.Role {
	case "OWNER", "MAINTAINER", "MEMBER", "GUEST":
	default:
		return fmt.Errorf("unknown role")
	}
	seen := map[string]bool{}
	for _, point := range role.Permissions {
		if seen[point] {
			return fmt.Errorf("duplicate permission")
		}
		seen[point] = true
		switch point {
		case "READ", "WRITE", "PUBLISH", "API_KEY_MANAGE":
		default:
			return fmt.Errorf("unknown permission")
		}
	}
	if role.Role == "OWNER" && len(seen) != 4 {
		return fmt.Errorf("owner permissions cannot be restricted")
	}
	return nil
}
