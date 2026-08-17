import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const authApi = vi.hoisted(() => ({
  login: vi.fn(),
  logout: vi.fn(),
  refreshSession: vi.fn(),
}))
const environmentApi = vi.hoisted(() => ({
  getMyPermissions: vi.fn(),
  listEnvironments: vi.fn(),
}))

vi.mock('@/api/auth', () => authApi)
vi.mock('@/api/environments', () => environmentApi)

import { clearAccessToken, getAccessToken } from '@/auth/token'
import { createStore } from '@/store'

describe('session store', () => {
  let store

  beforeEach(() => {
    store = createStore()
    authApi.login.mockResolvedValue({
      access_token: 'memory-only-token',
      expires_in: 900,
      user: { id: 7, username: 'admin', is_system_admin: false },
    })
    authApi.logout.mockResolvedValue()
    environmentApi.listEnvironments.mockResolvedValue({
      list: [{ id: 1, key: 'main', name: '主环境', enabled: true }],
    })
    environmentApi.getMyPermissions.mockResolvedValue({
      permissions: ['project.read'],
    })
  })

  afterEach(() => {
    clearAccessToken()
    vi.clearAllMocks()
  })

  it('登录令牌只进入内存令牌容器', async () => {
    await store.dispatch('session/login', {
      username: 'admin',
      password: 'secret',
    })

    expect(getAccessToken()).toBe('memory-only-token')
    expect(store.state.session.user.username).toBe('admin')
    expect(store.state.session).not.toHaveProperty('accessToken')
    expect(store.getters['session/authenticated']).toBe(true)
  })

  it('退出时清理会话和环境状态', async () => {
    await store.dispatch('session/login', {
      username: 'admin',
      password: 'secret',
    })
    await store.dispatch('session/logout')

    expect(getAccessToken()).toBeNull()
    expect(store.state.session.user).toBeNull()
    expect(store.state.environment.currentId).toBeNull()
  })
})
