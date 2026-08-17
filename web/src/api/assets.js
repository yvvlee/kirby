import axios from 'axios'

import client from './client'

const FORBIDDEN_UPLOAD_HEADERS = new Set([
  'authorization',
  'cookie',
  'proxy-authorization',
  'set-cookie',
  'x-kirby-api-key',
])

const SUPPORTED_UPLOAD_METHODS = new Set(['POST', 'PUT'])
const CANONICAL_UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
const CANONICAL_EXTENSION = /^\.[a-z0-9]+$/

const storageClient = axios.create({
  withCredentials: false,
  withXSRFToken: false,
  xsrfCookieName: null,
  xsrfHeaderName: null,
})

function positiveID(name, value) {
  const id = String(value)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}

function assetPath(environmentId, projectId, operation) {
  const environment = positiveID('environmentId', environmentId)
  const project = positiveID('projectId', projectId)
  return `/admin/environments/${environment}/projects/${project}/assets/${operation}`
}

function requireFile(file) {
  if (!(file instanceof Blob)) {
    throw new TypeError('asset file must be a Blob')
  }
  if (typeof file.name !== 'string' || file.name.length === 0) {
    throw new TypeError('asset file must have a name')
  }
  return file
}

function requireString(name, value, maximum = Infinity) {
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

function requireHTTPURL(name, value, { allowRelative = false } = {}) {
  const candidate = requireString(name, value)
  const relative = candidate.startsWith('/') && !candidate.startsWith('//')
  if (relative && allowRelative) {
    if (candidate.includes('\\')) {
      throw new TypeError(`${name} is not canonical`)
    }
    return candidate
  }
  let parsed
  try {
    parsed = new URL(candidate)
  } catch {
    throw new TypeError(`${name} must be an HTTP URL`)
  }
  if (
    (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') ||
    parsed.username ||
    parsed.password ||
    parsed.hash
  ) {
    throw new TypeError(`${name} must be an HTTP URL without credentials or fragment`)
  }
  return candidate
}

function safeUploadHeaders(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('asset upload headers are invalid')
  }
  const headers = {}
  Object.entries(value).forEach(([name, headerValue]) => {
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

function safeUploadFields(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('asset upload form fields are invalid')
  }
  const entries = Object.entries(value)
  if (entries.length > 100) {
    throw new TypeError('asset upload form has too many fields')
  }
  const fields = {}
  entries.forEach(([name, fieldValue]) => {
    const normalizedName = requireString('asset upload form field name', name, 255)
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
  name,
  value,
  environmentId,
  projectId,
  { allowTemporary = false } = {},
) {
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
  const separator = scoped[5].lastIndexOf('.')
  const objectId = scoped[5].slice(0, separator)
  const extension = scoped[5].slice(separator)
  if (!CANONICAL_UUID.test(objectId) || !CANONICAL_EXTENSION.test(extension)) {
    throw new TypeError(`${name} is not canonical`)
  }
  return key
}

function requireTicket(data, environmentId, projectId) {
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new TypeError('asset presign response is invalid')
  }
  const uploadMethod = requireString(
    'asset uploadMethod',
    data.uploadMethod,
    8,
  )
  if (!SUPPORTED_UPLOAD_METHODS.has(uploadMethod)) {
    throw new TypeError('asset uploadMethod is not supported')
  }
  const headers = safeUploadHeaders(data.headers)
  const formFields = safeUploadFields(data.formFields)
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
    data.objectKey,
    environmentId,
    projectId,
    { allowTemporary: true },
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
    uploadURL: requireHTTPURL('asset uploadUrl', data.uploadUrl, {
      allowRelative: true,
    }),
    uploadMethod,
    headers,
    formFields,
    expiresAt: requireString('asset expiresAt', data.expiresAt),
  }
}

function requireAsset(data, environmentId, projectId) {
  const asset = data?.asset
  if (!asset || typeof asset !== 'object' || Array.isArray(asset)) {
    throw new TypeError('asset complete response is invalid')
  }
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
    url: requireHTTPURL('asset url', asset.url, { allowRelative: true }),
    contentType: requireString('asset contentType', asset.contentType, 255),
    size: asset.size,
  }
}

export async function presignAsset(
  environmentId,
  projectId,
  file,
  options = {},
) {
  const asset = requireFile(file)
  const { data } = await client.post(
    assetPath(environmentId, projectId, 'presign'),
    {
      environment_id: Number(environmentId),
      project_id: Number(projectId),
      filename: asset.name,
      content_type: asset.type || 'application/octet-stream',
      size: asset.size,
    },
    { signal: options.signal },
  )
  return requireTicket(data, environmentId, projectId)
}

export async function uploadObject(ticket, file, options = {}) {
  const asset = requireFile(file)
  if (!ticket || typeof ticket !== 'object' || Array.isArray(ticket)) {
    throw new TypeError('asset upload ticket is invalid')
  }
  const uploadURL = requireHTTPURL('asset uploadUrl', ticket.uploadURL, {
    allowRelative: true,
  })
  const headers = safeUploadHeaders(ticket.headers)
  const uploadMethod = requireString('asset uploadMethod', ticket.uploadMethod, 8)
  if (uploadMethod === 'POST') {
    const form = new FormData()
    Object.entries(safeUploadFields(ticket.formFields)).forEach(
      ([name, value]) => form.append(name, value),
    )
    form.append('file', asset, asset.name)
    await storageClient.post(uploadURL, form, {
      headers,
      signal: options.signal,
      withCredentials: false,
      onUploadProgress: options.onUploadProgress,
    })
    return
  }
  if (uploadMethod !== 'PUT') {
    throw new TypeError('asset uploadMethod is not supported')
  }
  if (Object.keys(safeUploadFields(ticket.formFields)).length !== 0) {
    throw new TypeError('asset PUT upload must not contain form fields')
  }
  await storageClient.put(uploadURL, asset, {
    headers,
    signal: options.signal,
    withCredentials: false,
    onUploadProgress: options.onUploadProgress,
  })
}

export async function completeAsset(
  environmentId,
  projectId,
  objectKey,
  options = {},
) {
  const key = requireString('asset objectKey', objectKey, 1024)
  const { data } = await client.post(
    assetPath(environmentId, projectId, 'complete'),
    {
      environment_id: Number(environmentId),
      project_id: Number(projectId),
      object_key: key,
    },
    { signal: options.signal },
  )
  return requireAsset(data, environmentId, projectId)
}

export async function uploadAsset(environmentId, projectId, file, options = {}) {
  const ticket = await presignAsset(environmentId, projectId, file, options)
  await uploadObject(ticket, file, options)
  return completeAsset(environmentId, projectId, ticket.objectKey, options)
}
