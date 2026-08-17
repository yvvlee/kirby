import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const environmentApi = vi.hoisted(() => ({
  getMyPermissions: vi.fn(),
  listEnvironments: vi.fn(),
}))

vi.mock('@/api/environments', () => environmentApi)

import { registerEnvironmentScopeCleanup } from '@/auth/environment-scope'
import { createStore } from '@/store'

const environments = [
  { id: 11, key: 'east', name: '华东', enabled: true },
  { id: 22, key: 'west', name: '西部', enabled: true },
]

function deferred() {
  let resolve
  const promise = new Promise((done) => {
    resolve = done
  })
  return { promise, resolve }
}

describe('environment store', () => {
  let store
  let unregister

  beforeEach(() => {
    store = createStore()
    environmentApi.listEnvironments.mockResolvedValue({ list: environments })
    environmentApi.getMyPermissions.mockImplementation((environmentId) =>
      Promise.resolve({ permissions: [`environment:${environmentId}`] }),
    )
  })

  afterEach(() => {
    unregister?.()
    unregister = null
    vi.clearAllMocks()
  })

  it('加载动态环境并在切换前清理旧环境作用域', async () => {
    await store.dispatch('environment/loadAvailable')
    const cleanup = vi.fn()
    unregister = registerEnvironmentScopeCleanup('config-center-test', cleanup)

    await store.dispatch('environment/select', 22)

    expect(store.getters['environment/current']).toEqual(environments[1])
    expect(store.state.environment.permissions).toEqual(['environment:22'])
    expect(cleanup).toHaveBeenCalledWith({
      fromEnvironmentId: 11,
      toEnvironmentId: 22,
    })
  })

  it('拒绝选择未授权的环境', async () => {
    await store.dispatch('environment/loadAvailable')

    await expect(store.dispatch('environment/select', 99)).rejects.toThrow(
      'environment is not available: 99',
    )
  })

  it('没有当前环境时仍执行全量作用域清理', async () => {
    const cleanup = vi.fn()
    unregister = registerEnvironmentScopeCleanup('config-center-test', cleanup)

    await store.dispatch('environment/resetScope')

    expect(cleanup).toHaveBeenCalledWith({
      fromEnvironmentId: null,
      toEnvironmentId: null,
    })
  })

  it('退出清理后不接收在途环境列表响应', async () => {
    const pending = deferred()
    environmentApi.listEnvironments.mockReturnValueOnce(pending.promise)
    const loading = store.dispatch('environment/loadAvailable')

    await store.dispatch('environment/resetScope')
    pending.resolve({ list: environments })

    await expect(loading).resolves.toEqual([])
    expect(store.state.environment.available).toEqual([])
    expect(store.state.environment.currentId).toBeNull()
  })
})
