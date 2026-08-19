import axios, {
  type AxiosProgressEvent,
  type AxiosRequestConfig,
} from 'axios'

import client from './client'
import { positiveId, positiveIdentifier } from './environment-resource'
import type { ApiObject, Identifier } from './types'

const FORBIDDEN_UPLOAD_HEADERS = new Set([
  'authorization',
  'cookie',
  'proxy-authorization',
  'set-cookie',
  'x-kirby-api-key',
])
const SUPPORTED_UPLOAD_METHODS = new Set(['POST', 'PUT'])
const CANONICAL_UUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
const CANONICAL_EXTENSION = /^\.[a-z0-9]+$/

const storageClient = axios.create({
  withCredentials: false,
  withXSRFToken: false,
  xsrfCookieName: null as unknown as string,
  xsrfHeaderName: null as unknown as string,
})

export type AssetFile = Blob & { name: string }

export type AssetUploadTicket = {
  objectKey: string
  uploadURL: string
  uploadMethod: 'POST' | 'PUT'
  headers: Record<string, string>
  formFields: Record<string, string>
  expiresAt: string
}

export type UploadedAsset = ApiObject & {
  objectKey: string
  url: string
  contentType: string
  size: string | number
}

export type AssetUploadOptions = {
  signal?: AbortSignal
  onUploadProgress?: (event: AxiosProgressEvent) => void
}

function requestOptions(
  options: AssetUploadOptions,
): AxiosRequestConfig {
  const config: AxiosRequestConfig = {}
  if (options.signal) config.signal = options.signal
  if (options.onUploadProgress) {
    config.onUploadProgress = options.onUploadProgress
  }
  return config
}

function assetPath(
  environmentId: Identifier,
  projectId: Identifier,
  operation: string,
): string {
  return `/admin/environments/${positiveId(environmentId, 'environmentId')}/projects/${positiveId(projectId, 'projectId')}/assets/${operation}`
}

function requireFile(file: Blob): AssetFile {
  if (
    !(file instanceof Blob) ||
    !('name' in file) ||
    typeof file.name !== 'string' ||
    file.name.length === 0
  ) {
    throw new TypeError('asset file must have a name')
  }
  return file as AssetFile
}

function requireString(
  name: string,
  value: unknown,
  maximum = Number.POSITIVE_INFINITY,
): string {
  const hasControlCharacter =
    typeof value === 'string' &&
    [...value].some((character) => {
      const code = character.charCodeAt(0)
      return code <= 31 || code === 127
    })
  if (
    typeof value !== 'string' ||
    value.length === 0 ||
    value.length > maximum ||
    value.trim() !== value ||
    hasControlCharacter
  ) {
    throw new TypeError(`${name} is invalid`)
  }
  return value
}

function requireHttpUrl(
  name: string,
  value: unknown,
  allowRelative = false,
): string {
  const candidate = requireString(name, value)
  const relative = candidate.startsWith('/') && !candidate.startsWith('//')
  if (relative && allowRelative) {
    if (candidate.includes('\\')) {
      throw new TypeError(`${name} is not canonical`)
    }
    return candidate
  }
  let parsed: URL
  try {
    parsed = new URL(candidate)
  } catch {
    throw new TypeError(`${name} must be an HTTP URL`)
  }
  if (
    !['http:', 'https:'].includes(parsed.protocol) ||
    parsed.username ||
    parsed.password ||
    parsed.hash
  ) {
    throw new TypeError(
      `${name} must be an HTTP URL without credentials or fragment`,
    )
  }
  return candidate
}

function asUnknownRecord(name: string, value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new TypeError(`${name} is invalid`)
  }
  return value as Record<string, unknown>
}

function safeUploadHeaders(value: unknown): Record<string, string> {
  const source = asUnknownRecord('asset upload headers', value)
  const headers: Record<string, string> = {}
  Object.entries(source).forEach(([name, headerValue]) => {
    const normalizedName = requireString('asset upload header name', name)
    if (FORBIDDEN_UPLOAD_HEADERS.has(normalizedName.toLowerCase())) {
      throw new TypeError(`asset upload header ${normalizedName} is forbidden`)
    }
    headers[normalizedName] = requireString(
      `asset upload header ${normalizedName}`,
      headerValue,
    )
  })
  return headers
}

function safeUploadFields(value: unknown): Record<string, string> {
  const source = asUnknownRecord('asset upload form fields', value)
  const entries = Object.entries(source)
  if (entries.length > 100) {
    throw new TypeError('asset upload form has too many fields')
  }
  const fields: Record<string, string> = {}
  entries.forEach(([name, fieldValue]) => {
    const normalizedName = requireString(
      'asset upload form field name',
      name,
      255,
    )
    const lowerName = normalizedName.toLowerCase()
    if (FORBIDDEN_UPLOAD_HEADERS.has(lowerName) || lowerName === 'file') {
      throw new TypeError(`asset upload form field ${normalizedName} is forbidden`)
    }
    fields[normalizedName] = requireString(
      `asset upload form field ${normalizedName}`,
      fieldValue,
      65536,
    )
  })
  return fields
}

function requireScopedObjectKey(
  name: string,
  value: unknown,
  environmentId: Identifier,
  projectId: Identifier,
  allowTemporary = false,
): string {
  const key = requireString(name, value, 1024)
  const parts = key.split('/')
  const temporary = parts[0] === 'uploads'
  const scoped = temporary ? parts.slice(1) : parts
  if (
    (temporary && !allowTemporary) ||
    scoped.length !== 6 ||
    scoped[0] !== 'environments' ||
    scoped[1] !== String(environmentId) ||
    scoped[2] !== 'projects' ||
    scoped[3] !== String(projectId) ||
    scoped[4] !== 'assets'
  ) {
    throw new TypeError(`${name} has a mismatched environment or project scope`)
  }
  const fileName = scoped[5] ?? ''
  const separator = fileName.lastIndexOf('.')
  const objectId = fileName.slice(0, separator)
  const extension = fileName.slice(separator)
  if (!CANONICAL_UUID.test(objectId) || !CANONICAL_EXTENSION.test(extension)) {
    throw new TypeError(`${name} is not canonical`)
  }
  return key
}

function requireTicket(
  data: unknown,
  environmentId: Identifier,
  projectId: Identifier,
): AssetUploadTicket {
  const source = asUnknownRecord('asset presign response', data)
  const uploadMethod = requireString('asset uploadMethod', source.uploadMethod, 8)
  if (!SUPPORTED_UPLOAD_METHODS.has(uploadMethod)) {
    throw new TypeError('asset uploadMethod is not supported')
  }
  const headers = safeUploadHeaders(source.headers)
  const formFields = safeUploadFields(source.formFields)
  if (uploadMethod === 'POST' && Object.keys(formFields).length === 0) {
    throw new TypeError('asset POST upload form is empty')
  }
  if (uploadMethod === 'POST' && Object.keys(headers).length !== 0) {
    throw new TypeError('asset POST upload must not contain request headers')
  }
  if (uploadMethod === 'PUT' && Object.keys(formFields).length !== 0) {
    throw new TypeError('asset PUT upload must not contain form fields')
  }
  const objectKey = requireScopedObjectKey(
    'asset objectKey',
    source.objectKey,
    environmentId,
    projectId,
    true,
  )
  const temporary = objectKey.startsWith('uploads/')
  if (uploadMethod === 'POST' && !temporary) {
    throw new TypeError('asset POST upload requires a temporary objectKey')
  }
  if (uploadMethod === 'PUT' && temporary) {
    throw new TypeError('asset PUT upload requires a final objectKey')
  }
  if (uploadMethod === 'POST' && formFields.key !== objectKey) {
    throw new TypeError('asset POST upload form key does not match objectKey')
  }
  return {
    objectKey,
    uploadURL: requireHttpUrl('asset uploadUrl', source.uploadUrl, true),
    uploadMethod: uploadMethod as 'POST' | 'PUT',
    headers,
    formFields,
    expiresAt: requireString('asset expiresAt', source.expiresAt),
  }
}

function requireAsset(
  data: unknown,
  environmentId: Identifier,
  projectId: Identifier,
): UploadedAsset {
  const reply = asUnknownRecord('asset complete response', data)
  const asset = asUnknownRecord('asset complete response', reply.asset)
  const returnedKey = requireScopedObjectKey(
    'asset objectKey',
    asset.objectKey,
    environmentId,
    projectId,
  )
  const size = String(asset.size)
  if (!/^[1-9]\d*$/.test(size)) {
    throw new TypeError('asset complete response has an invalid size')
  }
  return {
    ...asset,
    objectKey: returnedKey,
    url: requireHttpUrl('asset url', asset.url, true),
    contentType: requireString('asset contentType', asset.contentType, 255),
    size: asset.size as string | number,
  }
}

export async function presignAsset(
  environmentId: Identifier,
  projectId: Identifier,
  file: Blob,
  options: AssetUploadOptions = {},
): Promise<AssetUploadTicket> {
  const asset = requireFile(file)
  const { data } = await client.post<unknown>(
    assetPath(environmentId, projectId, 'presign'),
    {
      environment_id: positiveIdentifier(environmentId, 'environmentId'),
      project_id: positiveIdentifier(projectId, 'projectId'),
      filename: asset.name,
      content_type: asset.type || 'application/octet-stream',
      size: asset.size,
    },
    requestOptions(options),
  )
  return requireTicket(data, environmentId, projectId)
}

export async function uploadObject(
  ticket: AssetUploadTicket,
  file: Blob,
  options: AssetUploadOptions = {},
): Promise<void> {
  const asset = requireFile(file)
  const uploadURL = requireHttpUrl('asset uploadUrl', ticket.uploadURL, true)
  const headers = safeUploadHeaders(ticket.headers)
  if (ticket.uploadMethod === 'POST') {
    const form = new FormData()
    Object.entries(safeUploadFields(ticket.formFields)).forEach(
      ([name, value]) => form.append(name, value),
    )
    form.append('file', asset, asset.name)
    await storageClient.post(uploadURL, form, {
      ...requestOptions(options),
      headers,
      withCredentials: false,
    })
    return
  }
  if (ticket.uploadMethod !== 'PUT') {
    throw new TypeError('asset uploadMethod is not supported')
  }
  if (Object.keys(safeUploadFields(ticket.formFields)).length !== 0) {
    throw new TypeError('asset PUT upload must not contain form fields')
  }
  await storageClient.put(uploadURL, asset, {
    ...requestOptions(options),
    headers,
    withCredentials: false,
  })
}

export async function completeAsset(
  environmentId: Identifier,
  projectId: Identifier,
  objectKey: string,
  options: AssetUploadOptions = {},
): Promise<UploadedAsset> {
  const key = requireString('asset objectKey', objectKey, 1024)
  const { data } = await client.post<unknown>(
    assetPath(environmentId, projectId, 'complete'),
    {
      environment_id: positiveIdentifier(environmentId, 'environmentId'),
      project_id: positiveIdentifier(projectId, 'projectId'),
      object_key: key,
    },
    requestOptions(options),
  )
  return requireAsset(data, environmentId, projectId)
}

export async function uploadAsset(
  environmentId: Identifier,
  projectId: Identifier,
  file: Blob,
  options: AssetUploadOptions = {},
): Promise<UploadedAsset> {
  const ticket = await presignAsset(environmentId, projectId, file, options)
  await uploadObject(ticket, file, options)
  return completeAsset(environmentId, projectId, ticket.objectKey, options)
}
