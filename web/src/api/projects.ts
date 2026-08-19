import { postEnvironmentOperation } from './environment-resource'
import type { ApiEntity, ApiListReply, ApiObject, Identifier } from './types'

export function createProject(
  environmentId: Identifier,
  project: ApiObject,
): Promise<ApiEntity> {
  return postEnvironmentOperation(environmentId, 'project', 'create', project)
}

export function updateProject(
  environmentId: Identifier,
  project: ApiObject,
): Promise<ApiEntity> {
  return postEnvironmentOperation(environmentId, 'project', 'update', project)
}

export function listProjects(
  environmentId: Identifier,
  filter: ApiObject = {},
): Promise<ApiListReply> {
  return postEnvironmentOperation(environmentId, 'project', 'list', filter)
}

export function getProject(
  environmentId: Identifier,
  projectId: Identifier,
): Promise<ApiEntity> {
  return postEnvironmentOperation(environmentId, 'project', 'detail', {
    id: projectId,
  })
}
