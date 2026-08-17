import client from './client'

function configPath(environmentId, operation) {
  const id = String(environmentId)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError('environmentId must be a positive integer')
  }
  return `/admin/environments/${id}/config/${operation}`
}

function requestBody(environmentId, value = {}) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('config request must be an object')
  }
  return { ...value, environment_id: environmentId }
}

async function post(environmentId, operation, value) {
  const { data } = await client.post(
    configPath(environmentId, operation),
    requestBody(environmentId, value),
  )
  return data
}

export function createConfig(environmentId, config) {
  return post(environmentId, 'create', config)
}

export function updateConfig(environmentId, config) {
  return post(environmentId, 'update', config)
}

export function updateConfigValue(environmentId, config) {
  return post(environmentId, 'updateValue', config)
}

export function listConfigs(environmentId, filter = {}) {
  return post(environmentId, 'list', filter)
}

export function getConfig(environmentId, configId) {
  return post(environmentId, 'detail', { id: configId })
}

export function deleteConfig(environmentId, configId) {
  return post(environmentId, 'delete', { id: configId })
}
