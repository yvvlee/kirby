import client from './client'
import type { ApiObject, Identifier } from './types'
export type PermissionRole = { role: string; permissions: string[] }
export type PermissionMatrix = ApiObject & { roles: PermissionRole[] }
const path = (id: Identifier) => `/admin/environments/${id}/permissions`
export async function getEnvironmentPermissionMatrix(id: Identifier) { const { data } = await client.get<PermissionMatrix>(path(id)); return data }
export async function updateEnvironmentPermissionMatrix(id: Identifier, roles: PermissionRole[]) { const { data } = await client.put<PermissionMatrix>(path(id), { environment_id: id, roles }); return data }
