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
  client.get.mockReset().mockResolvedValue({
    data: {
      list: [
        {
          id: '31',
          public_id: 'kirby_pk_31',
          name: 'production',
          secret_suffix: 'abcd',
          created_by: '1',
          created_at: '2026-08-18T05:43:01Z',
          last_used_at: '',
          revoked_at: '',
        },
      ],
    },
  })
  client.post.mockReset().mockResolvedValue({
    data: {
      api_key: {
        id: '31',
        public_id: 'kirby_pk_31',
        secret_suffix: 'abcd',
      },
      secret: 'secret',
    },
  })
})

describe('project API key contracts', () => {
  it('maps list, create, rotate, and revoke to protobuf paths', async () => {
    const listReply = await listProjectApiKeys(11, 7)
    const createReply = await createProjectApiKey(11, 7, 'production')
    const rotateReply = await rotateProjectApiKey(11, 7, 31)
    await revokeProjectApiKey(11, 7, 31)

    expect(listReply.list[0]).toMatchObject({
      publicId: 'kirby_pk_31',
      secretSuffix: 'abcd',
      createdBy: '1',
      createdAt: '2026-08-18T05:43:01Z',
      lastUsedAt: '',
      revokedAt: '',
    })
    expect(createReply).toMatchObject({
      apiKey: { id: '31', publicId: 'kirby_pk_31', secretSuffix: 'abcd' },
      secret: 'secret',
    })
    expect(rotateReply).toEqual(createReply)

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
