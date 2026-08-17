import client from './client'

function positiveId(value, name) {
  const id = String(value)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}

function rolePath(roleId, suffix = '') {
  return `/admin/roles/${positiveId(roleId, 'roleId')}${suffix}`
}

function environmentPath(environmentId, suffix = '') {
  return `/admin/environments/${positiveId(
    environmentId,
    'environmentId',
  )}${suffix}`
}

function positiveIds(values, name) {
  if (!Array.isArray(values)) {
    throw new TypeError(`${name} must be an array`)
  }
  const canonicalIds = values.map((value) => positiveId(value, name))
  if (new Set(canonicalIds).size !== canonicalIds.length) {
    throw new TypeError(`${name} must not contain duplicates`)
  }
  return [...values]
}

export async function listRoles() {
  const { data } = await client.get('/admin/roles')
  return data
}

export async function createRole(role) {
  const { data } = await client.post('/admin/roles', role)
  return data
}

export async function updateRole(roleId, role) {
  const { data } = await client.put(rolePath(roleId), {
    ...role,
    role_id: roleId,
  })
  return data
}

export async function deleteRole(roleId) {
  const { data } = await client.delete(rolePath(roleId))
  return data
}

export async function listPermissions() {
  const { data } = await client.get('/admin/permissions')
  return data
}

export async function updateRolePermissions(roleId, permissionIds) {
  const validatedIds = positiveIds(permissionIds, 'permissionIds')
  const { data } = await client.put(rolePath(roleId, '/permissions'), {
    role_id: roleId,
    permission_ids: validatedIds,
  })
  return data
}

export async function listEnvironmentMembers(environmentId) {
  const { data } = await client.get(
    environmentPath(environmentId, '/users'),
  )
  return data
}

export async function updateEnvironmentMemberRoles(
  environmentId,
  userId,
  roleIds,
) {
  const validatedRoleIds = positiveIds(roleIds, 'roleIds')
  const { data } = await client.put(
    environmentPath(
      environmentId,
      `/users/${positiveId(userId, 'userId')}/roles`,
    ),
    {
      environment_id: environmentId,
      user_id: userId,
      role_ids: validatedRoleIds,
    },
  )
  return data
}
