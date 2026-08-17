import client from './client'

function positiveId(name, value) {
  const id = String(value)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}

function scopePath(environmentId, projectId) {
  const environment = positiveId('environmentId', environmentId)
  const project = positiveId('projectId', projectId)
  return `/admin/environments/${environment}/projects/${project}/api-keys`
}

function keyPath(environmentId, projectId, keyId, operation = '') {
  const key = positiveId('keyId', keyId)
  const suffix = operation ? `/${operation}` : ''
  return `${scopePath(environmentId, projectId)}/${key}${suffix}`
}

function scopeBody(environmentId, projectId, value = {}) {
  return {
    ...value,
    environment_id: Number(positiveId('environmentId', environmentId)),
    project_id: Number(positiveId('projectId', projectId)),
  }
}

export async function listProjectApiKeys(environmentId, projectId) {
  const { data } = await client.get(scopePath(environmentId, projectId))
  return data
}

export async function createProjectApiKey(environmentId, projectId, name) {
  if (typeof name !== 'string' || name.length < 1 || name.length > 64) {
    throw new TypeError('apiKey name must contain 1 to 64 characters')
  }
  const { data } = await client.post(
    scopePath(environmentId, projectId),
    scopeBody(environmentId, projectId, { name }),
  )
  return data
}

export async function rotateProjectApiKey(
  environmentId,
  projectId,
  keyId,
) {
  const { data } = await client.post(
    keyPath(environmentId, projectId, keyId, 'rotate'),
    scopeBody(environmentId, projectId, {
      key_id: Number(positiveId('keyId', keyId)),
    }),
  )
  return data
}

export async function revokeProjectApiKey(
  environmentId,
  projectId,
  keyId,
) {
  const { data } = await client.delete(
    keyPath(environmentId, projectId, keyId),
  )
  return data
}
