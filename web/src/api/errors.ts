import { getLocale, t } from '@/i18n'
import { clientErrorMessages } from '@/i18n/client-errors'
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
  fallback = t('请求失败'),
): string {
  if (axios.isAxiosError(error)) {
    if (error.code === 'ERR_NETWORK') return t('网络连接失败，请稍后重试')
    if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') return t('请求超时，请稍后重试')
    const body = readErrorBody(error.response?.data)
    if (typeof body?.detail === 'string' && body.detail.length > 0) {
      return body.detail
    }
    if (typeof body?.message === 'string' && body.message.length > 0) {
      return body.message
    }
  }
  if (error instanceof Error && error.message.length > 0) {
    const localized = clientErrorMessages[error.message]
    if (getLocale() === 'zh-CN' && typeof localized === 'string') return localized
    return t(error.message)
  }
  return t(fallback)
}
