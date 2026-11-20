import { describe, expect, it, vi } from 'vitest'

import {
  clearEnvironmentScope,
  registerEnvironmentScopeCleanup,
} from '@/auth/environment-scope'

import { formatDiffValue } from './format-diff'
import {
  normalizeSnapshotList,
  parseSnapshotContent,
  snapshotStatusLabel,
} from './snapshots'

describe('snapshot model', () => {
  it('accepts only released and unreleased states', () => {
    expect(snapshotStatusLabel('RELEASED')).toBe('已发布')
    expect(() => snapshotStatusLabel('PENDING')).toThrow(
      '不支持的快照状态: PENDING',
    )
    expect(() =>
      normalizeSnapshotList({
        list: [{ status: 'REJECTED', tags: [] }],
        page: {},
      }),
    ).toThrow('不支持的快照状态: REJECTED')
  })

  it('parses JSON and rejects malformed content', () => {
    expect(parseSnapshotContent('{"enabled":true}')).toEqual({ enabled: true })
    expect(() => parseSnapshotContent('{')).toThrow(
      '快照内容不是合法 JSON',
    )
  })
})

describe('pure utilities', () => {
  it('formats stable JSON diff values', () => {
    expect(formatDiffValue({ enabled: true })).toBe(
      '{\n  "enabled": true\n}',
    )
    expect(formatDiffValue(undefined)).toBe('null')
    expect(() => formatDiffValue(() => undefined)).toThrow(
      '差异对比值无法转换为 JSON',
    )
  })

  it('runs registered environment cleanup handlers', async () => {
    const cleanup = vi.fn()
    const unregister = registerEnvironmentScopeCleanup('domain-test', cleanup)
    const change = { fromEnvironmentId: 1, toEnvironmentId: 2 }

    await clearEnvironmentScope(change)

    expect(cleanup).toHaveBeenCalledWith(change)
    unregister()
  })
})
