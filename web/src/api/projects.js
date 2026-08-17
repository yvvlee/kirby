import client from './client'

function projectPath(environmentId, operation) {
  const id = String(environmentId)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError('environmentId must be a positive integer')
  }
  return `/admin/environments/${id}/project/${operation}`
}

function requestBody(environmentId, value = {}) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('project request must be an object')
  }
  return { ...value, environment_id: environmentId }
}

async function post(environmentId, operation, value) {
  const { data } = await client.post(
    projectPath(environmentId, operation),
    requestBody(environmentId, value),
  )
  return data
}

export function createProject(environmentId, project) {
  return post(environmentId, 'create', project)
}

export function updateProject(environmentId, project) {
  return post(environmentId, 'update', project)
}

export function listProjects(environmentId, filter = {}) {
  return post(environmentId, 'list', filter)
}

export function getProject(environmentId, projectId) {
  return post(environmentId, 'detail', { id: projectId })
}
