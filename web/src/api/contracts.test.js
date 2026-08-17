import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('./client', () => ({ default: client }))

import { login, refreshSession } from './auth'
import { getMyPermissions, updateEnvironment } from './environments'

beforeEach(() => {
  client.get.mockResolvedValue({ data: {} })
  client.post.mockResolvedValue({ data: {} })
  client.put.mockResolvedValue({ data: {} })
})

describe('HTTP contract mapping', () => {
  it('登录和刷新使用公开契约且刷新不读取浏览器令牌', async () => {
    await login({ username: 'admin', password: 'secret' })
    await refreshSession()

    expect(client.post).toHaveBeenNthCalledWith(
      1,
      '/auth/login',
      { username: 'admin', password: 'secret' },
      { skipAccessToken: true, skipAuthRefresh: true },
    )
    expect(client.post).toHaveBeenNthCalledWith(
      2,
      '/auth/refresh',
      null,
      { skipAccessToken: true, skipAuthRefresh: true },
    )
  })

  it('环境 ID 只来自显式参数', async () => {
    await getMyPermissions(12)
    await updateEnvironment(12, {
      name: '华东',
      description: '',
      enabled: true,
      version: 3,
    })

    expect(client.get).toHaveBeenCalledWith(
      '/admin/environments/12/my-permissions',
    )
    expect(client.put).toHaveBeenCalledWith('/admin/environments/12', {
      environment_id: 12,
      name: '华东',
      description: '',
      enabled: true,
      version: 3,
    })
  })
})
