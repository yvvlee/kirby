package permission

// ProjectRole is a fixed project membership role.
type ProjectRole string

const (
	ProjectRoleOwner      ProjectRole = "OWNER"
	ProjectRoleMaintainer ProjectRole = "MAINTAINER"
	ProjectRoleMember     ProjectRole = "MEMBER"
	ProjectRoleGuest      ProjectRole = "GUEST"
)

// EnvironmentPermission is a fixed permission point within one environment.
type EnvironmentPermission string

const (
	EnvironmentPermissionRead         EnvironmentPermission = "READ"
	EnvironmentPermissionWrite        EnvironmentPermission = "WRITE"
	EnvironmentPermissionPublish      EnvironmentPermission = "PUBLISH"
	EnvironmentPermissionAPIKeyManage EnvironmentPermission = "API_KEY_MANAGE" // #nosec G101 -- permission identifier, not credential material.
)

// DefaultEnvironmentPermissions returns the permissions initialized for a role.
//
// Parameters: role identifies the fixed project role.
// Returns: the default environment permission points.
// Throws: none.
func DefaultEnvironmentPermissions(role ProjectRole) []EnvironmentPermission {
	switch role {
	case ProjectRoleOwner, ProjectRoleMaintainer:
		return []EnvironmentPermission{
			EnvironmentPermissionRead,
			EnvironmentPermissionWrite,
			EnvironmentPermissionPublish,
			EnvironmentPermissionAPIKeyManage,
		}
	case ProjectRoleMember:
		return []EnvironmentPermission{
			EnvironmentPermissionRead,
			EnvironmentPermissionWrite,
		}
	case ProjectRoleGuest:
		return []EnvironmentPermission{
			EnvironmentPermissionRead,
		}
	default:
		return nil
	}
}

// CanGrant reports whether actorRole can grant targetRole.
//
// Parameters: actorRole is the current member role; targetRole is the role to grant.
// Returns: true when the target role is no higher than the actor role.
// Throws: none.
func CanGrant(actorRole, targetRole ProjectRole) bool {
	if actorRole != ProjectRoleOwner && actorRole != ProjectRoleMaintainer {
		return false
	}
	targetRank := projectRoleRank(targetRole)
	return targetRank > 0 && targetRank <= projectRoleRank(actorRole)
}

func projectRoleRank(role ProjectRole) int {
	switch role {
	case ProjectRoleOwner:
		return 4
	case ProjectRoleMaintainer:
		return 3
	case ProjectRoleMember:
		return 2
	case ProjectRoleGuest:
		return 1
	default:
		return 0
	}
}
