import client from './client'
import { positiveId, positiveIdentifier } from './environment-resource'
import type { ApiObject, Identifier } from './types'

export type ImportConflictStrategy = 'FAIL' | 'REPLACE'

export type SnapshotImportRequest = {
  source_environment_id: Identifier
  source_snapshot_id: Identifier
  target_project_id: Identifier
  target_config_id?: Identifier
  conflict_strategy: ImportConflictStrategy
  idempotency_key: string
  description: string
  tags: unknown[]
}

function requireVersion(version: number): number {
  if (!Number.isSafeInteger(version) || version < 0) {
    throw new TypeError('version must be a non-negative integer')
  }
  return version
}

function publicationPath(
  environmentId: Identifier,
  snapshotId: Identifier,
  operation: string,
): string {
  return `/admin/environments/${positiveId(environmentId, 'environmentId')}/snapshots/${positiveId(snapshotId, 'snapshotId')}/${operation}`
}

function publicationBody(
  environmentId: Identifier,
  snapshotId: Identifier,
  version: number,
): ApiObject {
  return {
    environment_id: positiveIdentifier(environmentId, 'environmentId'),
    snapshot_id: positiveIdentifier(snapshotId, 'snapshotId'),
    version: requireVersion(version),
  }
}

export async function publishSnapshot(
  environmentId: Identifier,
  snapshotId: Identifier,
  version: number,
): Promise<ApiObject> {
  const { data } = await client.post<ApiObject>(
    publicationPath(environmentId, snapshotId, 'publish'),
    publicationBody(environmentId, snapshotId, version),
  )
  return data
}

export async function unpublishSnapshot(
  environmentId: Identifier,
  snapshotId: Identifier,
  version: number,
): Promise<ApiObject> {
  const { data } = await client.post<ApiObject>(
    publicationPath(environmentId, snapshotId, 'unpublish'),
    publicationBody(environmentId, snapshotId, version),
  )
  return data
}

export async function exportSnapshot(
  sourceEnvironmentId: Identifier,
  snapshotId: Identifier,
): Promise<ApiObject> {
  const environment = positiveId(sourceEnvironmentId, 'sourceEnvironmentId')
  const snapshot = positiveId(snapshotId, 'snapshotId')
  const { data } = await client.get<ApiObject>(
    `/admin/environments/${environment}/snapshots/${snapshot}/export`,
  )
  return data
}

export function createImportIdempotencyKey(): string {
  if (typeof globalThis.crypto?.randomUUID !== 'function') {
    throw new Error('Web Crypto randomUUID is required for snapshot import')
  }
  return `kirby-import-${globalThis.crypto.randomUUID()}`
}

function importBody(
  targetEnvironmentId: Identifier,
  request: SnapshotImportRequest,
): ApiObject {
  const sourceEnvironmentId = positiveIdentifier(
    request.source_environment_id,
    'sourceEnvironmentId',
  )
  const sourceSnapshotId = positiveIdentifier(
    request.source_snapshot_id,
    'sourceSnapshotId',
  )
  const targetProjectId = positiveIdentifier(
    request.target_project_id,
    'targetProjectId',
  )
  if (
    request.conflict_strategy === 'REPLACE' &&
    request.target_config_id === undefined
  ) {
    throw new TypeError('targetConfigId is required for REPLACE')
  }
  if (
    request.idempotency_key.length < 16 ||
    request.idempotency_key.length > 128
  ) {
    throw new TypeError('idempotencyKey must contain 16 to 128 characters')
  }
  if (request.description.length < 2 || request.description.length > 255) {
    throw new TypeError('description must contain 2 to 255 characters')
  }
  if (!Array.isArray(request.tags)) {
    throw new TypeError('snapshot import tags must be an array')
  }

  const body: ApiObject = {
    source_environment_id: sourceEnvironmentId,
    source_snapshot_id: sourceSnapshotId,
    target_project_id: targetProjectId,
    description: request.description,
    tags: [...request.tags],
    idempotency_key: request.idempotency_key,
    conflict_strategy: request.conflict_strategy,
    target_environment_id: positiveIdentifier(
      targetEnvironmentId,
      'targetEnvironmentId',
    ),
  }
  if (request.target_config_id !== undefined) {
    body.target_config_id = positiveIdentifier(
      request.target_config_id,
      'targetConfigId',
    )
  }
  return body
}

export async function importSnapshot(
  targetEnvironmentId: Identifier,
  request: SnapshotImportRequest,
): Promise<ApiObject> {
  const environment = positiveId(targetEnvironmentId, 'targetEnvironmentId')
  const { data } = await client.post<ApiObject>(
    `/admin/environments/${environment}/snapshot-imports`,
    importBody(targetEnvironmentId, request),
  )
  return data
}
