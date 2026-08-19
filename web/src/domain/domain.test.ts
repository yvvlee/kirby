import { describe, expect, it, vi } from 'vitest'

import {
  clearEnvironmentScope,
  registerEnvironmentScopeCleanup,
} from '@/auth/environment-scope'

import { actorAccess, assignableRoles, SYSTEM_PERMISSIONS } from './access'
import { formatDiffValue } from './format-diff'
import {
  normalizeSnapshotList,
  parseSnapshotContent,
  snapshotStatusLabel,
} from './snapshots'

describe('permission model', () => {
  it('limits environment administrators to member management', () => {
    expect(
      actorAccess({ permissions: [SYSTEM_PERMISSIONS.manageMembers] }),
    ).toEqual({
      manageEnvironments: false,
      manageUsers: false,
      manageRoles: false,
      manageMembers: true,
    })
  })

  it('filters roles containing system permissions', () => {
    const roles = [
      { id: 1, permissions: [{ key: 'project:read' }] },
      { id: 2, permissions: [{ key: 'system:user:manage' }] },
      { id: 3 },
    ]
    expect(assignableRoles(roles, false).map((role) => role.id)).toEqual([1])
  })
})

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
