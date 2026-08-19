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
  rotateProjectApiKey,
} from './api-keys'
import {
  importSnapshot,
  publishSnapshot,
  type SnapshotImportRequest,
} from './snapshot-imports'

beforeEach(() => {
  vi.clearAllMocks()
  client.delete.mockResolvedValue({ data: {} })
  client.get.mockResolvedValue({ data: { list: [] } })
  client.post.mockResolvedValue({ data: {} })
})

describe('project API key contracts', () => {
  it('normalizes server field names without retaining the secret', async () => {
    client.get.mockResolvedValue({
      data: {
        list: [
          {
            id: 7,
            public_id: 'pub-1',
            secret_suffix: 'abcd',
            created_at: '2026-08-19T00:00:00Z',
          },
        ],
      },
    })

    await expect(listProjectApiKeys(3, 5)).resolves.toMatchObject({
      list: [
        {
          id: 7,
          publicId: 'pub-1',
          secretSuffix: 'abcd',
          createdAt: '2026-08-19T00:00:00Z',
        },
      ],
    })
  })

  it('preserves 64-bit IDs and rejects malformed list items', async () => {
    const largeId = '9223372036854775807'
    client.get.mockResolvedValueOnce({
      data: { list: [{ id: largeId, created_by: largeId }] },
    })
    await expect(listProjectApiKeys(3, 5)).resolves.toMatchObject({
      list: [{ id: largeId, createdBy: largeId }],
    })

    client.get.mockResolvedValueOnce({ data: { list: [{ id: 0 }] } })
    await expect(listProjectApiKeys(3, 5)).rejects.toThrow(
      'api key list contains an invalid item',
    )
  })

  it('scopes create and rotate requests explicitly', async () => {
    await createProjectApiKey(3, 5, 'deploy')
    await rotateProjectApiKey(3, 5, 7)

    expect(client.post).toHaveBeenNthCalledWith(
      1,
      '/admin/environments/3/projects/5/api-keys',
      { environment_id: 3, project_id: 5, name: 'deploy' },
    )
    expect(client.post).toHaveBeenNthCalledWith(
      2,
      '/admin/environments/3/projects/5/api-keys/7/rotate',
      { environment_id: 3, project_id: 5, key_id: 7 },
    )
  })
})

describe('snapshot publication and import contracts', () => {
  const request: SnapshotImportRequest = {
    source_environment_id: 1,
    source_snapshot_id: 2,
    target_project_id: 3,
    conflict_strategy: 'FAIL',
    idempotency_key: 'kirby-import-1234567890',
    description: 'import release',
    tags: ['release'],
  }

  it('includes scope and version in publication', async () => {
    await publishSnapshot(4, 8, 2)
    expect(client.post).toHaveBeenCalledWith(
      '/admin/environments/4/snapshots/8/publish',
      { environment_id: 4, snapshot_id: 8, version: 2 },
    )
  })

  it('builds an idempotent import request', async () => {
    await importSnapshot(4, request)
    expect(client.post).toHaveBeenCalledWith(
      '/admin/environments/4/snapshot-imports',
      {
        source_environment_id: 1,
        source_snapshot_id: 2,
        target_project_id: 3,
        target_environment_id: 4,
        conflict_strategy: 'FAIL',
        idempotency_key: 'kirby-import-1234567890',
        description: 'import release',
        tags: ['release'],
      },
    )
  })

  it('requires a target config when replacing', async () => {
    await expect(
      importSnapshot(4, { ...request, conflict_strategy: 'REPLACE' }),
    ).rejects.toThrow('targetConfigId is required for REPLACE')
    expect(client.post).not.toHaveBeenCalled()
  })

  it('preserves 64-bit IDs in snapshot requests', async () => {
    const largeId = '9223372036854775807'
    await publishSnapshot(largeId, largeId, 2)
    await importSnapshot(largeId, {
      ...request,
      source_environment_id: largeId,
      source_snapshot_id: largeId,
      target_project_id: largeId,
    })

    expect(client.post).toHaveBeenNthCalledWith(
      1,
      `/admin/environments/${largeId}/snapshots/${largeId}/publish`,
      { environment_id: largeId, snapshot_id: largeId, version: 2 },
    )
    expect(client.post).toHaveBeenNthCalledWith(
      2,
      `/admin/environments/${largeId}/snapshot-imports`,
      expect.objectContaining({
        source_environment_id: largeId,
        source_snapshot_id: largeId,
        target_project_id: largeId,
        target_environment_id: largeId,
      }),
    )
  })
})
