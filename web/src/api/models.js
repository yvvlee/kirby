import client from './client'

function modelPath(environmentId, operation) {
  const id = String(environmentId)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError('environmentId must be a positive integer')
  }
  return `/admin/environments/${id}/structure/${operation}`
}

function requestBody(environmentId, value = {}) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('model request must be an object')
  }
  return { ...value, environment_id: environmentId }
}

async function post(environmentId, operation, value) {
  const { data } = await client.post(
    modelPath(environmentId, operation),
    requestBody(environmentId, value),
  )
  return data
}

export function createModel(environmentId, model) {
  return post(environmentId, 'create', model)
}

export function updateModel(environmentId, model) {
  return post(environmentId, 'update', model)
}

export function listModels(environmentId, filter) {
  return post(environmentId, 'list', filter)
}

export function deleteModel(environmentId, modelId) {
  return post(environmentId, 'delete', { id: modelId })
}
