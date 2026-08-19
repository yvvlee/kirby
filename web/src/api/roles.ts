import client from './client'
import { positiveId } from './environment-resource'
import type {
  ApiEntity,
  ApiListReply,
  ApiObject,
  Identifier,
} from './types'

function rolePath(roleId: Identifier, suffix = ''): string {
  return `/admin/roles/${positiveId(roleId, 'roleId')}${suffix}`
}

function environmentPath(environmentId: Identifier, suffix = ''): string {
  return `/admin/environments/${positiveId(environmentId, 'environmentId')}${suffix}`
}

function positiveIds(values: Identifier[], name: string): Identifier[] {
  if (!Array.isArray(values)) {
    throw new TypeError(`${name} must be an array`)
  }
  const canonicalIds = values.map((value) => positiveId(value, name))
  if (new Set(canonicalIds).size !== canonicalIds.length) {
    throw new TypeError(`${name} must not contain duplicates`)
  }
  return [...values]
}

export async function listRoles(): Promise<ApiListReply> {
  const { data } = await client.get<ApiListReply>('/admin/roles')
  return data
}

export async function createRole(role: ApiObject): Promise<ApiEntity> {
  const { data } = await client.post<ApiEntity>('/admin/roles', role)
  return data
}

export async function updateRole(
  roleId: Identifier,
  role: ApiObject,
): Promise<ApiEntity> {
  const { data } = await client.put<ApiEntity>(rolePath(roleId), {
    ...role,
    role_id: roleId,
  })
  return data
}

export async function deleteRole(roleId: Identifier): Promise<ApiObject> {
  const { data } = await client.delete<ApiObject>(rolePath(roleId))
  return data
}

export async function listPermissions(): Promise<ApiListReply> {
  const { data } = await client.get<ApiListReply>('/admin/permissions')
  return data
}

export async function updateRolePermissions(
  roleId: Identifier,
  permissionIds: Identifier[],
): Promise<ApiObject> {
  const validatedIds = positiveIds(permissionIds, 'permissionIds')
  const { data } = await client.put<ApiObject>(
    rolePath(roleId, '/permissions'),
    { role_id: roleId, permission_ids: validatedIds },
  )
  return data
}

export async function listEnvironmentMembers(
  environmentId: Identifier,
): Promise<ApiListReply> {
  const { data } = await client.get<ApiListReply>(
    environmentPath(environmentId, '/users'),
  )
  return data
}

export async function updateEnvironmentMemberRoles(
  environmentId: Identifier,
  userId: Identifier,
  roleIds: Identifier[],
): Promise<ApiObject> {
  const validatedRoleIds = positiveIds(roleIds, 'roleIds')
  const { data } = await client.put<ApiObject>(
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
