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
  const projectId = environment.project_id
  if (typeof projectId !== 'number' && typeof projectId !== 'string') throw new TypeError('project_id is required')
  const { data } = await client.post<Environment>(
    `/admin/projects/${positiveId(projectId, 'projectId')}/environments`,
    environment,
  )
  return data
}

export async function updateEnvironment(
  environmentId: Identifier,
  environment: ApiObject,
): Promise<Environment> {
  const projectId = environment.project_id
  if (typeof projectId !== 'number' && typeof projectId !== 'string') throw new TypeError('project_id is required')
  const { data } = await client.put<Environment>(
    `/admin/projects/${positiveId(projectId, 'projectId')}/environments/${positiveId(environmentId, 'environmentId')}`,
    { ...environment, environment_id: environmentId, project_id: projectId },
  )
  return data
}
