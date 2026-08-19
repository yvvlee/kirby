import client from './client'
import { positiveId } from './environment-resource'
import type {
  ApiListReply,
  ApiObject,
  Environment,
  Identifier,
  PermissionReply,
} from './types'

function environmentPath(environmentId: Identifier, suffix = ''): string {
  return `/admin/environments/${positiveId(environmentId, 'environmentId')}${suffix}`
}

export async function listEnvironments(): Promise<ApiListReply<Environment>> {
  const { data } = await client.get<ApiListReply<Environment>>(
    '/admin/environments',
  )
  return data
}

export async function getMyPermissions(
  environmentId: Identifier,
): Promise<PermissionReply> {
  const { data } = await client.get<PermissionReply>(
    environmentPath(environmentId, '/my-permissions'),
  )
  return data
}

export async function createEnvironment(
  environment: ApiObject,
): Promise<Environment> {
  const { data } = await client.post<Environment>(
    '/admin/environments',
    environment,
  )
  return data
}

export async function updateEnvironment(
  environmentId: Identifier,
  environment: ApiObject,
): Promise<Environment> {
  const { data } = await client.put<Environment>(
    environmentPath(environmentId),
    { ...environment, environment_id: environmentId },
  )
  return data
}
