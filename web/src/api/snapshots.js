import client from './client'

function snapshotPath(environmentId, operation) {
  const id = String(environmentId)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError('environmentId must be a positive integer')
  }
  return `/admin/environments/${id}/snapshot/${operation}`
}

function requestBody(environmentId, value = {}) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('snapshot request must be an object')
  }
  return { ...value, environment_id: environmentId }
}

async function post(environmentId, operation, value) {
  const { data } = await client.post(
    snapshotPath(environmentId, operation),
    requestBody(environmentId, value),
  )
  return data
}

export function createSnapshot(environmentId, snapshot) {
  return post(environmentId, 'create', snapshot)
}

export function previewCreatingSnapshot(environmentId, configId) {
  return post(environmentId, 'previewCreating', { config_id: configId })
}

export function deleteSnapshot(environmentId, snapshotId) {
  return post(environmentId, 'delete', { id: snapshotId })
}

export function getSnapshot(environmentId, snapshotId) {
  return post(environmentId, 'detail', { id: snapshotId })
}

export function loadSnapshot(environmentId, snapshotId, configId) {
  return post(environmentId, 'load', {
    id: snapshotId,
    config_id: configId,
  })
}

export function getCurrentSnapshot(environmentId, configId) {
  return post(environmentId, 'current', { config_id: configId })
}

export function getReleasedSnapshot(environmentId, configId) {
  return post(environmentId, 'released', { config_id: configId })
}

export function listSnapshots(environmentId, filter) {
  return post(environmentId, 'list', filter)
}
