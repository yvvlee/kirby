import { describe, expect, it, vi } from 'vitest'

import { createNavigationGuard } from './guards'

function createStore(getters) {
  return {
    dispatch: vi.fn().mockResolvedValue(true),
    getters,
  }
}

function route(meta, overrides = {}) {
  return {
    fullPath: '/settings',
    matched: [{ meta }],
    name: 'settings',
    query: {},
    ...overrides,
  }
}

describe('navigation guard', () => {
  it('未登录访问受保护路由时转到登录页', async () => {
    const store = createStore({
      'session/authenticated': false,
      'session/systemAdmin': false,
      'environment/hasPermission': () => false,
    })
    const next = vi.fn()

    await createNavigationGuard(store)(
      route({ requiresAuth: true }),
      route({}),
      next,
    )

    expect(next).toHaveBeenCalledWith({
      name: 'login',
      query: { redirect: '/settings' },
    })
  })

  it('无环境权限时转到 403 页面', async () => {
    const store = createStore({
      'session/authenticated': true,
      'session/systemAdmin': false,
      'environment/hasPermission': () => false,
    })
    const next = vi.fn()

    await createNavigationGuard(store)(
      route({ requiresAuth: true, permission: 'role.write' }),
      route({}),
      next,
    )

    expect(next).toHaveBeenCalledWith({ name: 'forbidden' })
  })
})
