import axios from 'axios'

import client from './client'

const FORBIDDEN_UPLOAD_HEADERS = new Set([
  'authorization',
  'cookie',
  'proxy-authorization',
  'set-cookie',
  'x-kirby-api-key',
])

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

function requireTicket(data) {
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new TypeError('asset presign response is invalid')
  }
  return {
    objectKey: requireString('asset objectKey', data.objectKey, 1024),
    uploadURL: requireHTTPURL('asset uploadUrl', data.uploadUrl, {
      allowRelative: true,
    }),
    headers: safeUploadHeaders(data.headers),
    expiresAt: requireString('asset expiresAt', data.expiresAt),
  }
}

function requireAsset(data, objectKey) {
  const asset = data?.asset
  if (!asset || typeof asset !== 'object' || Array.isArray(asset)) {
    throw new TypeError('asset complete response is invalid')
  }
  const returnedKey = requireString('asset objectKey', asset.objectKey, 1024)
  if (returnedKey !== objectKey) {
    throw new TypeError('asset complete response has a mismatched objectKey')
  }
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
  return requireTicket(data)
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
  return requireAsset(data, key)
}

export async function uploadAsset(environmentId, projectId, file, options = {}) {
  const ticket = await presignAsset(environmentId, projectId, file, options)
  await uploadObject(ticket, file, options)
  return completeAsset(environmentId, projectId, ticket.objectKey, options)
}
