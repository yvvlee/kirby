import client from './client'

const IMPORT_CONFLICT_STRATEGIES = new Set(['FAIL', 'REPLACE'])

function positiveId(name, value) {
  const id = String(value)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}

function requireObject(name, value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError(`${name} must be an object`)
  }
  return value
}

function requireVersion(version) {
  if (!Number.isSafeInteger(version) || version < 0) {
    throw new TypeError('version must be a non-negative integer')
  }
  return version
}

function publicationPath(environmentId, snapshotId, operation) {
  const environment = positiveId('environmentId', environmentId)
  const snapshot = positiveId('snapshotId', snapshotId)
  return `/admin/environments/${environment}/snapshots/${snapshot}/${operation}`
}

function publicationBody(environmentId, snapshotId, version) {
  return {
    environment_id: Number(positiveId('environmentId', environmentId)),
    snapshot_id: Number(positiveId('snapshotId', snapshotId)),
    version: requireVersion(version),
  }
}

export async function publishSnapshot(environmentId, snapshotId, version) {
  const { data } = await client.post(
    publicationPath(environmentId, snapshotId, 'publish'),
    publicationBody(environmentId, snapshotId, version),
  )
  return data
}

export async function unpublishSnapshot(environmentId, snapshotId, version) {
  const { data } = await client.post(
    publicationPath(environmentId, snapshotId, 'unpublish'),
    publicationBody(environmentId, snapshotId, version),
  )
  return data
}

export async function exportSnapshot(sourceEnvironmentId, snapshotId) {
  const environment = positiveId(
    'sourceEnvironmentId',
    sourceEnvironmentId,
  )
  const snapshot = positiveId('snapshotId', snapshotId)
  const { data } = await client.get(
    `/admin/environments/${environment}/snapshots/${snapshot}/export`,
  )
  return data
}

export function createImportIdempotencyKey() {
  if (typeof globalThis.crypto?.randomUUID !== 'function') {
    throw new Error('Web Crypto randomUUID is required for snapshot import')
  }
  return `kirby-import-${globalThis.crypto.randomUUID()}`
}

function importBody(targetEnvironmentId, request) {
  request = requireObject('snapshot import request', request)
  const sourceEnvironmentId = Number(
    positiveId('sourceEnvironmentId', request.source_environment_id),
  )
  const sourceSnapshotId = Number(
    positiveId('sourceSnapshotId', request.source_snapshot_id),
  )
  const targetProjectId = Number(
    positiveId('targetProjectId', request.target_project_id),
  )
  if (!IMPORT_CONFLICT_STRATEGIES.has(request.conflict_strategy)) {
    throw new TypeError('conflictStrategy must be FAIL or REPLACE')
  }
  if (
    request.conflict_strategy === 'REPLACE' &&
    request.target_config_id === undefined
  ) {
    throw new TypeError('targetConfigId is required for REPLACE')
  }
  const idempotencyKey = request.idempotency_key
  if (
    typeof idempotencyKey !== 'string' ||
    idempotencyKey.length < 16 ||
    idempotencyKey.length > 128
  ) {
    throw new TypeError('idempotencyKey must contain 16 to 128 characters')
  }
  const description = request.description
  if (
    typeof description !== 'string' ||
    description.length < 2 ||
    description.length > 255
  ) {
    throw new TypeError('description must contain 2 to 255 characters')
  }
  if (!Array.isArray(request.tags)) {
    throw new TypeError('snapshot import tags must be an array')
  }

  const body = {
    source_environment_id: sourceEnvironmentId,
    source_snapshot_id: sourceSnapshotId,
    target_project_id: targetProjectId,
    description,
    tags: [...request.tags],
    idempotency_key: idempotencyKey,
    conflict_strategy: request.conflict_strategy,
    target_environment_id: Number(
      positiveId('targetEnvironmentId', targetEnvironmentId),
    ),
  }
  if (request.target_config_id !== undefined) {
    body.target_config_id = Number(
      positiveId('targetConfigId', request.target_config_id),
    )
  }
  return body
}

export async function importSnapshot(targetEnvironmentId, request) {
  const environment = positiveId(
    'targetEnvironmentId',
    targetEnvironmentId,
  )
  const { data } = await client.post(
    `/admin/environments/${environment}/snapshot-imports`,
    importBody(targetEnvironmentId, request),
  )
  return data
}
