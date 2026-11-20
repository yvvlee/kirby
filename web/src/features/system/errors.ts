import axios from 'axios'

import { getApiErrorMessage } from '@/api/errors'
import { t } from '@/i18n'

export function actionErrorMessage(error: unknown, action: string): string {
  const response = typeof error === 'object' && error !== null && 'response' in error &&
    typeof error.response === 'object' && error.response !== null
    ? error.response as Record<string, unknown>
    : null
  if ((axios.isAxiosError(error) && error.response?.status === 403) || response?.status === 403) {
    return t('没有权限{{action}}。当前页面已保留。', { action })
  }
  const data = response?.data
  if (typeof data === 'object' && data !== null && 'message' in data && typeof data.message === 'string') {
    return data.message
  }
  return getApiErrorMessage(error, t('{{action}}失败', { action }))
}

export function requireList<T>(reply: unknown, name: string): T[] {
  if (
    typeof reply !== 'object' ||
    reply === null ||
    !('list' in reply) ||
    !Array.isArray(reply.list)
  ) {
    throw new TypeError(t('响应缺少列表：{{name}}', { name }))
  }
  return reply.list as T[]
}
