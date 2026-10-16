import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))
vi.mock('./client', () => ({ default: client }))

import { createInitialAdministrator, getSetupStatus } from './setup'

beforeEach(() => vi.resetAllMocks())

describe('installation API', () => {
  it('reads status without access tokens or automatic refresh', async () => {
    client.get.mockResolvedValue({ data: { initialized: false } })

    await expect(getSetupStatus()).resolves.toEqual({ initialized: false })
    expect(client.get).toHaveBeenCalledWith('/setup/status', {
      skipAuthRefresh: true,
      skipAccessToken: true,
    })
  })

  it.each([null, {}, { initialized: 'false' }])('rejects invalid status: %j', async (data) => {
    client.get.mockResolvedValue({ data })
    await expect(getSetupStatus()).rejects.toThrow('初始化状态响应无效')
  })

  it('creates the administrator anonymously without swallowing conflicts', async () => {
    const administrator = {
      username: 'owner',
      display_name: '管理员',
      password: 'long-initial-password',
    }
    const conflict = new Error('already initialized')
    client.post.mockRejectedValue(conflict)

    await expect(createInitialAdministrator(administrator)).rejects.toBe(conflict)
    expect(client.post).toHaveBeenCalledWith('/setup/admin', administrator, {
      skipAuthRefresh: true,
      skipAccessToken: true,
    })
  })
})
