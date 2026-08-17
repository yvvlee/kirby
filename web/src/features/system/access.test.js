import { describe, expect, it } from 'vitest'

import {
  SYSTEM_PERMISSIONS,
  actorAccess,
  assignableRoles,
} from './access'
import { actionErrorMessage } from './errors'

describe('system administration access matrix', () => {
  it('system administrators see all system and environment administration', () => {
    expect(actorAccess({ systemAdmin: true, permissions: [] })).toEqual({
      manageEnvironments: true,
      manageUsers: true,
      manageRoles: true,
      manageMembers: true,
    })
  })

  it('environment administrators only see current-environment members', () => {
    expect(
      actorAccess({
        systemAdmin: false,
        permissions: [SYSTEM_PERMISSIONS.manageMembers],
      }),
    ).toEqual({
      manageEnvironments: false,
      manageUsers: false,
      manageRoles: false,
      manageMembers: true,
    })
  })

  it.each([
    ['ordinary member', ['project:read', 'config:write']],
    ['member without permissions', []],
  ])('%s does not see administration write entrances', (_, permissions) => {
    expect(actorAccess({ systemAdmin: false, permissions })).toEqual({
      manageEnvironments: false,
      manageUsers: false,
      manageRoles: false,
      manageMembers: false,
    })
  })

  it('environment administrators cannot assign roles containing system permissions', () => {
    const roles = [
      { id: 1, permissions: [{ key: 'project:read' }] },
      { id: 2, permissions: [{ key: 'system:user:manage' }] },
      { id: 3 },
      { id: 4, permissions: [{ description: 'missing key' }] },
    ]
    expect(assignableRoles(roles, false).map((role) => role.id)).toEqual([1])
    expect(assignableRoles(roles, true)).toEqual(roles)
  })

  it('403 produces an explicit in-page error', () => {
    const error = { response: { status: 403 } }
    expect(actionErrorMessage(error, '保存角色')).toBe(
      '没有权限保存角色。当前页面已保留。',
    )
  })
})
