import { getApiErrorMessage } from '@/api/errors'

export const SNAPSHOT_TAG_OPTIONS = Object.freeze([
  { label: 'Release', value: 'RELEASE' },
  { label: 'Hotfix', value: 'HOTFIX' },
  { label: 'Review', value: 'REVIEW' },
  { label: 'Debug', value: 'DEBUG' },
  { label: 'Demo', value: 'DEMO' },
  { label: 'Reuse', value: 'REUSE' },
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
  const label = STATUS_LABELS[status as SnapshotStatus]
  if (!label) {
    throw new Error(`不支持的快照状态: ${status}`)
  }
  return label
}

export function normalizeSnapshotList(reply: unknown): {
  list: SnapshotListItem[]
  page: Record<string, unknown>
} {
  if (typeof reply !== 'object' || reply === null || Array.isArray(reply)) {
    throw new TypeError('快照列表响应缺少 list')
  }
  const source = reply as Record<string, unknown>
  if (!Array.isArray(source.list)) {
    throw new TypeError('快照列表响应缺少 list')
  }
  if (
    typeof source.page !== 'object' ||
    source.page === null ||
    Array.isArray(source.page)
  ) {
    throw new TypeError('快照列表响应缺少 page')
  }
  const list = source.list.map((snapshot) => {
    if (
      typeof snapshot !== 'object' ||
      snapshot === null ||
      Array.isArray(snapshot)
    ) {
      throw new TypeError('快照列表项必须是对象')
    }
    const item = snapshot as Record<string, unknown>
    if (typeof item.status !== 'string') {
      throw new TypeError('快照列表项缺少 status')
    }
    snapshotStatusLabel(item.status)
    if (!Array.isArray(item.tags)) {
      throw new TypeError('快照列表项缺少 tags')
    }
    return item as SnapshotListItem
  })
  return { list, page: source.page as Record<string, unknown> }
}

export function parseSnapshotContent(content: string): unknown {
  if (content.length === 0) {
    throw new TypeError('快照内容必须是非空 JSON 字符串')
  }
  try {
    return JSON.parse(content) as unknown
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error)
    throw new SyntaxError(`快照内容不是合法 JSON: ${message}`, {
      cause: error,
    })
  }
}

export function snapshotActionError(
  error: unknown,
  action: string,
  suffix = '当前页面已保留。',
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
    return `没有权限${action}。${suffix}`
  }
  return getApiErrorMessage(error, `${action}失败`)
}
