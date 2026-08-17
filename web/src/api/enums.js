import client from './client'

function enumPath(environmentId, operation) {
  const id = String(environmentId)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError('environmentId must be a positive integer')
  }
  return `/admin/environments/${id}/enum/${operation}`
}

function requestBody(environmentId, value = {}) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('enum request must be an object')
  }
  return { ...value, environment_id: environmentId }
}

async function post(environmentId, operation, value) {
  const { data } = await client.post(
    enumPath(environmentId, operation),
    requestBody(environmentId, value),
  )
  return data
}

export function createEnum(environmentId, configEnum) {
  return post(environmentId, 'create', configEnum)
}

export function updateEnum(environmentId, configEnum) {
  return post(environmentId, 'update', configEnum)
}

export function listEnums(environmentId, filter) {
  return post(environmentId, 'list', filter)
}

export function deleteEnum(environmentId, enumId) {
  return post(environmentId, 'delete', { id: enumId })
}
