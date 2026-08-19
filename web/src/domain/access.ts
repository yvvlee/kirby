export const SYSTEM_PERMISSIONS = Object.freeze({
  manageEnvironments: 'system:environment:manage',
  manageUsers: 'system:user:manage',
  manageRoles: 'system:role:manage',
  manageMembers: 'environment:member:manage',
})

export type ActorAccess = {
  manageEnvironments: boolean
  manageUsers: boolean
  manageRoles: boolean
  manageMembers: boolean
}

export function actorAccess({
  systemAdmin = false,
  permissions = [],
}: {
  systemAdmin?: boolean
  permissions?: string[]
} = {}): ActorAccess {
  const hasPermission = (permission: string) => permissions.includes(permission)
  return {
    manageEnvironments: systemAdmin,
    manageUsers: systemAdmin,
    manageRoles: systemAdmin,
    manageMembers:
      systemAdmin || hasPermission(SYSTEM_PERMISSIONS.manageMembers),
  }
}

type RolePermission = { key?: string }
type RoleWithPermissions = { permissions?: RolePermission[] }

export function assignableRoles<T extends RoleWithPermissions>(
  roles: T[],
  systemAdmin: boolean,
): T[] {
  if (!Array.isArray(roles)) {
    throw new TypeError('role response does not contain list')
  }
  if (systemAdmin) {
    return roles
  }
  return roles.filter((role) =>
    Array.isArray(role.permissions) &&
    role.permissions.every(
      (permission) =>
        typeof permission.key === 'string' &&
        permission.key.length > 0 &&
        !permission.key.startsWith('system:'),
    ),
  )
}
