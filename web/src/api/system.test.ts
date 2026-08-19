import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({
  delete: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))
vi.mock('./client', () => ({ default: client }))

import {
  listEnvironmentMembers,
  listPermissions,
  updateEnvironmentMemberRoles,
  updateRolePermissions,
} from './roles'
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
    await updateUser(12, { display_name: 'Alice', version: 3 })
    await updateUserPassword(12, 'another-long-password')
    await updateUserStatus(12, false, 4)

    expect(client.post).toHaveBeenCalledWith('/admin/users', {
      username: 'alice',
      password: 'long-password',
    })
    expect(client.put).toHaveBeenNthCalledWith(1, '/admin/users/12', {
      user_id: 12,
      display_name: 'Alice',
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

  it('uses explicit environment IDs for member roles', async () => {
    await listEnvironmentMembers(23)
    await updateEnvironmentMemberRoles(23, 45, [2, 3])

    expect(client.get).toHaveBeenCalledWith('/admin/environments/23/users')
    expect(client.put).toHaveBeenCalledWith(
      '/admin/environments/23/users/45/roles',
      { environment_id: 23, user_id: 45, role_ids: [2, 3] },
    )
  })

  it('submits only validated permission IDs', async () => {
    await listPermissions()
    await updateRolePermissions(8, [1, 7, 16])

    expect(client.get).toHaveBeenCalledWith('/admin/permissions')
    expect(client.put).toHaveBeenCalledWith('/admin/roles/8/permissions', {
      role_id: 8,
      permission_ids: [1, 7, 16],
    })
  })

  it('rejects invalid IDs before an HTTP request', async () => {
    await expect(listEnvironmentMembers('../other')).rejects.toThrow(
      'environmentId must be a positive integer',
    )
    await expect(updateUser(0, {})).rejects.toThrow(
      'userId must be a positive integer',
    )
    await expect(updateRolePermissions(1, [2, 2])).rejects.toThrow(
      'permissionIds must not contain duplicates',
    )
    expect(client.get).not.toHaveBeenCalled()
    expect(client.put).not.toHaveBeenCalled()
  })
})
