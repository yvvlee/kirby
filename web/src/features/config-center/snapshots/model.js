export const SNAPSHOT_TAG_OPTIONS = Object.freeze([
  { label: 'Release', value: 'RELEASE' },
  { label: 'Hotfix', value: 'HOTFIX' },
  { label: 'Review', value: 'REVIEW' },
  { label: 'Debug', value: 'DEBUG' },
  { label: 'Demo', value: 'DEMO' },
  { label: 'Reuse', value: 'REUSE' },
])

const STATUS_LABELS = Object.freeze({
  RELEASED: '已发布',
  UNRELEASED: '未发布',
})

export function snapshotStatusLabel(status) {
  const label = STATUS_LABELS[status]
  if (!label) {
    throw new Error(`不支持的快照状态: ${status}`)
  }
  return label
}

export function normalizeSnapshotList(reply) {
  if (!Array.isArray(reply?.list)) {
    throw new TypeError('快照列表响应缺少 list')
  }
  if (!reply.page || typeof reply.page !== 'object') {
    throw new TypeError('快照列表响应缺少 page')
  }
  const list = reply.list.map((snapshot) => {
    if (!snapshot || typeof snapshot !== 'object') {
      throw new TypeError('快照列表项必须是对象')
    }
    snapshotStatusLabel(snapshot.status)
    if (!Array.isArray(snapshot.tags)) {
      throw new TypeError('快照列表项缺少 tags')
    }
    return snapshot
  })
  return { list, page: reply.page }
}

export function parseSnapshotContent(content) {
  if (typeof content !== 'string' || content.length === 0) {
    throw new TypeError('快照内容必须是非空 JSON 字符串')
  }
  try {
    return JSON.parse(content)
  } catch (error) {
    throw new SyntaxError(`快照内容不是合法 JSON: ${error.message}`, {
      cause: error,
    })
  }
}

export function snapshotActionError(error, action, suffix = '当前页面已保留。') {
  if (error?.response?.status === 403) {
    return `没有权限${action}。${suffix}`
  }
  return error?.response?.data?.message || error?.message || `${action}失败`
}
