import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({
  delete: vi.fn(),
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('./client', () => ({ default: client }))

import {
  createProjectApiKey,
  listProjectApiKeys,
  revokeProjectApiKey,
  rotateProjectApiKey,
} from './api-keys'

beforeEach(() => {
  client.delete.mockReset().mockResolvedValue({ data: {} })
  client.get.mockReset().mockResolvedValue({ data: { list: [] } })
  client.post.mockReset().mockResolvedValue({ data: { apiKey: {}, secret: 'secret' } })
})

describe('project API key contracts', () => {
  it('maps list, create, rotate, and revoke to protobuf paths', async () => {
    await listProjectApiKeys(11, 7)
    await createProjectApiKey(11, 7, 'production')
    await rotateProjectApiKey(11, 7, 31)
    await revokeProjectApiKey(11, 7, 31)

    const base = '/admin/environments/11/projects/7/api-keys'
    expect(client.get).toHaveBeenCalledWith(base)
    expect(client.post).toHaveBeenNthCalledWith(1, base, {
      environment_id: 11,
      project_id: 7,
      name: 'production',
    })
    expect(client.post).toHaveBeenNthCalledWith(2, `${base}/31/rotate`, {
      environment_id: 11,
      project_id: 7,
      key_id: 31,
    })
    expect(client.delete).toHaveBeenCalledWith(`${base}/31`)
  })

  it('rejects implicit or invalid scope IDs', async () => {
    await expect(listProjectApiKeys(0, 7)).rejects.toThrow(
      'environmentId must be a positive integer',
    )
    await expect(rotateProjectApiKey(11, 7, 0)).rejects.toThrow(
      'keyId must be a positive integer',
    )
    expect(client.get).not.toHaveBeenCalled()
    expect(client.post).not.toHaveBeenCalled()
  })
})
