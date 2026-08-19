import axios from 'axios'

type ErrorBody = {
  detail?: unknown
  message?: unknown
}

function readErrorBody(value: unknown): ErrorBody | null {
  return typeof value === 'object' && value !== null ? (value as ErrorBody) : null
}

export function getApiErrorMessage(
  error: unknown,
  fallback = '请求失败',
): string {
  if (axios.isAxiosError(error)) {
    const body = readErrorBody(error.response?.data)
    if (typeof body?.detail === 'string' && body.detail.length > 0) {
      return body.detail
    }
    if (typeof body?.message === 'string' && body.message.length > 0) {
      return body.message
    }
  }
  if (error instanceof Error && error.message.length > 0) {
    return error.message
  }
  return fallback
}
