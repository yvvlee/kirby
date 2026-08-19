import client from './client'
import { positiveId, positiveIdentifier } from './environment-resource'
import { isIdentifier, type ApiListReply, type ApiObject, type Identifier } from './types'

export type ProjectApiKey = ApiObject & {
  id: Identifier
  publicId?: string
  secretSuffix?: string
  createdBy?: Identifier
  createdAt?: string
  lastUsedAt?: string | null
  revokedAt?: string | null
}

export type ApiKeySecretReply = ApiObject & {
  apiKey?: ProjectApiKey
  secret?: string
}

function scopePath(environmentId: Identifier, projectId: Identifier): string {
  return `/admin/environments/${positiveId(environmentId, 'environmentId')}/projects/${positiveId(projectId, 'projectId')}/api-keys`
}

function keyPath(
  environmentId: Identifier,
  projectId: Identifier,
  keyId: Identifier,
  operation = '',
): string {
  const suffix = operation ? `/${operation}` : ''
  return `${scopePath(environmentId, projectId)}/${positiveId(keyId, 'keyId')}${suffix}`
}

function scopeBody(
  environmentId: Identifier,
  projectId: Identifier,
  value: ApiObject = {},
): ApiObject {
  return {
    ...value,
    environment_id: positiveIdentifier(environmentId, 'environmentId'),
    project_id: positiveIdentifier(projectId, 'projectId'),
  }
}

function normalizeProjectApiKey(value: unknown): ProjectApiKey | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return undefined
  }
  const source = value as Record<string, unknown>
  if (!isIdentifier(source.id)) return undefined
  const normalized: ProjectApiKey = {
    ...source,
    id: source.id,
  }
  if (typeof source.public_id === 'string') normalized.publicId = source.public_id
  if (typeof source.secret_suffix === 'string') normalized.secretSuffix = source.secret_suffix
  if (isIdentifier(source.created_by)) normalized.createdBy = source.created_by
  if (typeof source.created_at === 'string') normalized.createdAt = source.created_at
  if (typeof source.last_used_at === 'string' || source.last_used_at === null) {
    normalized.lastUsedAt = source.last_used_at
  }
  if (typeof source.revoked_at === 'string' || source.revoked_at === null) {
    normalized.revokedAt = source.revoked_at
  }
  return normalized
}

function normalizeListReply(value: unknown): ApiListReply<ProjectApiKey> {
  if (
    typeof value !== 'object' ||
    value === null ||
    Array.isArray(value) ||
    !('list' in value) ||
    !Array.isArray(value.list)
  ) {
    throw new TypeError('api key list response does not contain list')
  }
  const source = value as ApiObject & { list: unknown[] }
  const list = source.list.map(normalizeProjectApiKey)
  if (list.some((item) => item === undefined)) {
    throw new TypeError('api key list contains an invalid item')
  }
  return { ...source, list: list as ProjectApiKey[] }
}

function normalizeSecretReply(value: unknown): ApiKeySecretReply {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new TypeError('api key secret response is invalid')
  }
  const source = value as ApiObject & { api_key?: unknown }
  const normalized: ApiKeySecretReply = { ...source }
  const apiKey = normalizeProjectApiKey(source.api_key)
  if (apiKey) normalized.apiKey = apiKey
  return normalized
}

export async function listProjectApiKeys(
  environmentId: Identifier,
  projectId: Identifier,
): Promise<ApiListReply<ProjectApiKey>> {
  const { data } = await client.get<unknown>(scopePath(environmentId, projectId))
  return normalizeListReply(data)
}

export async function createProjectApiKey(
  environmentId: Identifier,
  projectId: Identifier,
  name: string,
): Promise<ApiKeySecretReply> {
  if (name.length < 1 || name.length > 64) {
    throw new TypeError('apiKey name must contain 1 to 64 characters')
  }
  const { data } = await client.post<unknown>(
    scopePath(environmentId, projectId),
    scopeBody(environmentId, projectId, { name }),
  )
  return normalizeSecretReply(data)
}

export async function rotateProjectApiKey(
  environmentId: Identifier,
  projectId: Identifier,
  keyId: Identifier,
): Promise<ApiKeySecretReply> {
  const { data } = await client.post<unknown>(
    keyPath(environmentId, projectId, keyId, 'rotate'),
    scopeBody(environmentId, projectId, {
      key_id: positiveIdentifier(keyId, 'keyId'),
    }),
  )
  return normalizeSecretReply(data)
}

export async function revokeProjectApiKey(
  environmentId: Identifier,
  projectId: Identifier,
  keyId: Identifier,
): Promise<ApiObject> {
  const { data } = await client.delete<ApiObject>(
    keyPath(environmentId, projectId, keyId),
  )
  return data
}
