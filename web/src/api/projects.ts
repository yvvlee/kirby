import client from './client'
import type { ApiEntity, ApiListReply, ApiObject, Identifier } from './types'

export function createProject(
  _environmentId: Identifier | null,
  project: ApiObject,
): Promise<ApiEntity> {
  return client.post<ApiEntity>('/admin/projects', project).then(({ data }) => data)
}

export function updateProject(
  _environmentId: Identifier | null,
  project: ApiObject,
): Promise<ApiEntity> {
  const id = project.id
  if (typeof id !== 'number' && typeof id !== 'string') throw new TypeError('project.id is required')
  return client.put<ApiEntity>(`/admin/projects/${id}`, project).then(({ data }) => data)
}

export function listProjects(
  environmentId: Identifier | null,
  filter: ApiObject = {},
): Promise<ApiListReply> {
  const params = environmentId === null ? filter : { ...filter, environment_id: environmentId }
  return client.get<ApiListReply>('/admin/projects', { params }).then(({ data }) => data)
}

export function getProject(
  environmentId: Identifier | null,
  projectId: Identifier,
): Promise<ApiEntity> {
  const request = environmentId === null
    ? client.get<ApiEntity>(`/admin/projects/${projectId}`)
    : client.get<ApiEntity>(`/admin/projects/${projectId}`, { params: { environment_id: environmentId } })
  return request.then(({ data }) => data)
}
