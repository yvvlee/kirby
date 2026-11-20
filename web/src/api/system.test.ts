import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({
  delete: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))
vi.mock('./client', () => ({ default: client }))

import {
  createUser,
  updateUser,
  updateUserPassword,
  updateUserStatus,
} from './users'

beforeEach(() => {
  vi.clearAllMocks()
  Object.values(client).forEach((method) => {
    method.mockResolvedValue({ data: {} })
  })
})

describe('system administration API contracts', () => {
  it('maps global user operations to fixed endpoints', async () => {
    await createUser({ username: 'alice', password: 'long-password' })
    await updateUser(12, { nickname: 'Alice', version: 3 })
    await updateUserPassword(12, 'another-long-password')
    await updateUserStatus(12, false, 4)

    expect(client.post).toHaveBeenCalledWith('/admin/users', {
      username: 'alice',
      password: 'long-password',
    })
    expect(client.put).toHaveBeenNthCalledWith(1, '/admin/users/12', {
      user_id: 12,
      nickname: 'Alice',
      version: 3,
    })
    expect(client.put).toHaveBeenNthCalledWith(
      2,
      '/admin/users/12/password',
      { user_id: 12, password: 'another-long-password' },
    )
    expect(client.put).toHaveBeenNthCalledWith(
      3,
      '/admin/users/12/status',
      { user_id: 12, enabled: false, version: 4 },
    )
  })

})
