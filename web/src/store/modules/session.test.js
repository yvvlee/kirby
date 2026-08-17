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

import {
  clearAccessToken,
  getAccessToken,
  startAccessTokenSession,
} from '@/auth/token'
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
    authApi.refreshSession.mockImplementation(async () => {
      const reply = {
        access_token: 'refreshed-memory-token',
        expires_in: 900,
        user: { id: 7, username: 'admin', is_system_admin: false },
      }
      startAccessTokenSession(reply.access_token)
      return reply
    })
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

  it('退出请求未完成时也立即使本地会话失效', async () => {
    let resolveLogout
    authApi.logout.mockReturnValue(
      new Promise((resolve) => {
        resolveLogout = resolve
      }),
    )
    await store.dispatch('session/login', {
      username: 'admin',
      password: 'secret',
    })

    const logout = store.dispatch('session/logout')

    expect(getAccessToken()).toBeNull()
    expect(store.state.session.user).toBeNull()
    resolveLogout()
    await logout
  })

  it('并发初始化只执行一次刷新和环境加载', async () => {
    let resolveRefresh
    const refreshReply = new Promise((resolve) => {
      resolveRefresh = () => {
        const reply = {
          access_token: 'bootstrap-token',
          expires_in: 900,
          user: { id: 9, username: 'reader', is_system_admin: false },
        }
        startAccessTokenSession(reply.access_token)
        resolve(reply)
      }
    })
    authApi.refreshSession.mockReturnValue(refreshReply)

    const first = store.dispatch('session/bootstrap')
    const second = store.dispatch('session/bootstrap')
    expect(authApi.refreshSession).toHaveBeenCalledTimes(1)

    resolveRefresh()
    await expect(Promise.all([first, second])).resolves.toEqual([true, true])
    expect(environmentApi.listEnvironments).toHaveBeenCalledTimes(1)
    expect(store.state.session.user.username).toBe('reader')
  })

  it('初始化失败不会清除期间完成的新登录', async () => {
    let rejectRefresh
    authApi.refreshSession.mockReturnValue(
      new Promise((_resolve, reject) => {
        rejectRefresh = reject
      }),
    )
    const bootstrap = store.dispatch('session/bootstrap')

    await store.dispatch('session/login', {
      username: 'admin',
      password: 'secret',
    })
    rejectRefresh(new Error('stale bootstrap failed'))

    await expect(bootstrap).resolves.toBe(true)
    expect(getAccessToken()).toBe('memory-only-token')
    expect(store.state.session.user.username).toBe('admin')
    expect(store.getters['session/authenticated']).toBe(true)
  })

  it('初始化自己的环境加载失败时会清除刚刷新的会话', async () => {
    environmentApi.listEnvironments.mockRejectedValueOnce(
      new Error('environment list failed'),
    )

    await expect(store.dispatch('session/bootstrap')).rejects.toThrow(
      'environment list failed',
    )

    expect(getAccessToken()).toBeNull()
    expect(store.state.session.user).toBeNull()
    expect(store.state.environment.currentId).toBeNull()
  })
})
