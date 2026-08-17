import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('./client', () => ({ default: client }))

import {
  exportSnapshot,
  importSnapshot,
  publishSnapshot,
  unpublishSnapshot,
} from './snapshot-imports'

beforeEach(() => {
  client.get.mockReset().mockResolvedValue({ data: { snapshot: {} } })
  client.post.mockReset().mockResolvedValue({ data: { snapshot: {} } })
})

describe('snapshot publication and transfer API contracts', () => {
  it('uses dedicated publication endpoints', async () => {
    await publishSnapshot(11, 31, 4)
    await unpublishSnapshot(11, 31, 5)

    expect(client.post).toHaveBeenNthCalledWith(
      1,
      '/admin/environments/11/snapshots/31/publish',
      { environment_id: 11, snapshot_id: 31, version: 4 },
    )
    expect(client.post).toHaveBeenNthCalledWith(
      2,
      '/admin/environments/11/snapshots/31/unpublish',
      { environment_id: 11, snapshot_id: 31, version: 5 },
    )
  })

  it('exports a source snapshot from its explicit environment', async () => {
    await exportSnapshot(11, 31)

    expect(client.get).toHaveBeenCalledWith(
      '/admin/environments/11/snapshots/31/export',
    )
  })

  it('sends the idempotency key only in the protobuf request body', async () => {
    const request = {
      source_environment_id: 11,
      source_snapshot_id: 31,
      target_project_id: 7,
      target_config_id: 9,
      description: '同步快照',
      tags: ['REUSE'],
      idempotency_key: 'kirby-import-fixed-12345',
      conflict_strategy: 'REPLACE',
      target_environment_id: 99,
    }

    await importSnapshot(22, request)

    expect(client.post).toHaveBeenCalledWith(
      '/admin/environments/22/snapshot-imports',
      {
        ...request,
        target_environment_id: 22,
      },
    )
  })

  it('fails before sending an invalid replacement request', async () => {
    await expect(
      importSnapshot(22, {
        source_environment_id: 11,
        source_snapshot_id: 31,
        target_project_id: 7,
        description: '同步快照',
        tags: [],
        idempotency_key: 'kirby-import-fixed-12345',
        conflict_strategy: 'REPLACE',
      }),
    ).rejects.toThrow('targetConfigId is required for REPLACE')
    expect(client.post).not.toHaveBeenCalled()
  })
})
