import { localizedError } from '@/i18n/errors'
import { getApiErrorMessage } from '@/api/errors'
import { t } from '@/i18n'

export const SNAPSHOT_TAG_OPTIONS = Object.freeze([
  { label: '发布版本', value: 'RELEASE' },
  { label: '热修复', value: 'HOTFIX' },
  { label: '评审', value: 'REVIEW' },
  { label: '调试', value: 'DEBUG' },
  { label: '演示', value: 'DEMO' },
  { label: '复用', value: 'REUSE' },
])

const STATUS_LABELS = {
  RELEASED: '已发布',
  UNRELEASED: '未发布',
} as const

export type SnapshotStatus = keyof typeof STATUS_LABELS
export type SnapshotListItem = Record<string, unknown> & {
  status: SnapshotStatus
  tags: unknown[]
}

export function snapshotStatusLabel(status: string): string {
  const sourceLabel = STATUS_LABELS[status as SnapshotStatus]
  if (!sourceLabel) {
    throw localizedError('不支持的快照状态: {{status}}', { status })
  }
  return t(sourceLabel)
}

export function normalizeSnapshotList(reply: unknown): {
  list: SnapshotListItem[]
  page: Record<string, unknown>
} {
  if (typeof reply !== 'object' || reply === null || Array.isArray(reply)) {
    throw localizedError('快照列表响应缺少 list', {}, TypeError)
  }
  const source = reply as Record<string, unknown>
  if (!Array.isArray(source.list)) {
    throw localizedError('快照列表响应缺少 list', {}, TypeError)
  }
  if (
    typeof source.page !== 'object' ||
    source.page === null ||
    Array.isArray(source.page)
  ) {
    throw localizedError('快照列表响应缺少 page', {}, TypeError)
  }
  const list = source.list.map((snapshot) => {
    if (
      typeof snapshot !== 'object' ||
      snapshot === null ||
      Array.isArray(snapshot)
    ) {
      throw localizedError('快照列表项必须是对象', {}, TypeError)
    }
    const item = snapshot as Record<string, unknown>
    if (typeof item.status !== 'string') {
      throw localizedError('快照列表项缺少 status', {}, TypeError)
    }
    snapshotStatusLabel(item.status)
    if (!Array.isArray(item.tags)) {
      throw localizedError('快照列表项缺少 tags', {}, TypeError)
    }
    return item as SnapshotListItem
  })
  return { list, page: source.page as Record<string, unknown> }
}

export function parseSnapshotContent(content: string): unknown {
  if (content.length === 0) {
    throw localizedError('快照内容必须是非空 JSON 字符串', {}, TypeError)
  }
  try {
    return JSON.parse(content) as unknown
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error)
    const localized = localizedError(
      '快照内容不是合法 JSON: {{message}}',
      { message },
      SyntaxError,
    )
    Object.defineProperty(localized, 'cause', { value: error })
    throw localized
  }
}

export function snapshotActionError(
  error: unknown,
  action: string,
  suffix = t('当前页面已保留。'),
): string {
  if (
    typeof error === 'object' &&
    error !== null &&
    'response' in error &&
    typeof error.response === 'object' &&
    error.response !== null &&
    'status' in error.response &&
    error.response.status === 403
  ) {
    return t('没有权限{{action}}。{{suffix}}', { action, suffix })
  }
  return getApiErrorMessage(error, t('{{action}}失败', { action }))
}
