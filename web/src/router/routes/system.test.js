import { describe, expect, it, vi } from 'vitest'

import { createRouter } from '@/router'

function mockStore() {
  return {
    dispatch: vi.fn().mockResolvedValue(true),
    getters: {
      'session/authenticated': true,
      'session/systemAdmin': true,
      'environment/hasPermission': () => true,
    },
  }
}

describe('system routes', () => {
  it('keeps system pages under the authenticated AppLayout route', () => {
    const router = createRouter(mockStore(), { mode: 'abstract' })
    const matched = router.match('/system/users').matched

    expect(matched).toHaveLength(3)
    expect(matched[0].meta.requiresAuth).toBe(true)
    expect(matched[2].name).toBe('system-users')
    expect(matched[2].meta.permission).toBe('system:user:manage')
  })

  it('marks environment members with the environment-scoped permission', () => {
    const router = createRouter(mockStore(), { mode: 'abstract' })
    const matched = router.match('/system/members').matched
    expect(matched[2].meta.permission).toBe('environment:member:manage')
  })
})
