export const SYSTEM_PERMISSIONS = Object.freeze({
  manageEnvironments: 'system:environment:manage',
  manageUsers: 'system:user:manage',
  manageRoles: 'system:role:manage',
  manageMembers: 'environment:member:manage',
})

export function actorAccess({ systemAdmin = false, permissions = [] } = {}) {
  const declaredPermissions = Array.isArray(permissions) ? permissions : []
  const hasPermission = (permission) =>
    declaredPermissions.includes(permission)
  return {
    manageEnvironments: systemAdmin,
    manageUsers: systemAdmin,
    manageRoles: systemAdmin,
    manageMembers:
      systemAdmin || hasPermission(SYSTEM_PERMISSIONS.manageMembers),
  }
}

export function assignableRoles(roles, systemAdmin) {
  if (!Array.isArray(roles)) {
    throw new TypeError('role response does not contain list')
  }
  if (systemAdmin) {
    return roles
  }
  return roles.filter((role) => {
    if (!Array.isArray(role.permissions)) {
      return false
    }
    const permissions = role.permissions
    return permissions.every(
      (permission) =>
        typeof permission.key === 'string' &&
        permission.key.length > 0 &&
        !permission.key.startsWith('system:'),
    )
  })
}
